package braid

import (
	"encoding/hex"
	"fmt"
	"sync"
)

// refKey returns a canonical string key for a MessageRef, suitable for use
// as a map key.
func refKey(ref *MessageRef) string {
	return hex.EncodeToString(ref.GetSha256V1())
}

// vertex is an incorporated message with resolved parent and children pointers
// for efficient in-memory traversal.
type vertex struct {
	msg      *Message
	ref      *MessageRef
	key      string
	parents  []*vertex
	children []*vertex
}

// Store holds the message DAG, including incorporated messages, pending
// messages awaiting parents, and the frontier (messages with no children).
type Store struct {
	mu sync.RWMutex

	// Incorporated messages.
	vertices map[string]*vertex   // refKey → vertex
	frontier map[*vertex]struct{} // vertices with no children

	// Pending messages awaiting missing parents.
	pending   map[string]*Message            // refKey → message
	blockedBy map[string]map[string]struct{} // missing parent refKey → set of pending refKeys waiting on it
	missing   map[string]int                 // pending refKey → count of missing parents
	wanted    map[string]struct{}            // refKeys we need to request from peers
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		vertices:  make(map[string]*vertex),
		frontier:  make(map[*vertex]struct{}),
		pending:   make(map[string]*Message),
		blockedBy: make(map[string]map[string]struct{}),
		missing:   make(map[string]int),
		wanted:    make(map[string]struct{}),
	}
}

// AddResult describes the outcome of a Store.Add call.
type AddResult struct {
	Ref              *MessageRef
	IsNew            bool
	IsPending        bool
	MissingAncestors []*MessageRef // transitive missing ancestors (only if IsPending)
}

// Add attempts to incorporate a message into the store. If parents are
// missing, the message is buffered as pending and the missing parent refs
// are added to the wanted set.
func (s *Store) Add(msg *Message) (AddResult, error) {
	ref, err := HashMessage(msg)
	if err != nil {
		return AddResult{}, fmt.Errorf("hashing message: %w", err)
	}
	key := refKey(ref)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip duplicates.
	if _, ok := s.vertices[key]; ok {
		return AddResult{Ref: ref}, nil
	}
	if _, ok := s.pending[key]; ok {
		return AddResult{Ref: ref}, nil
	}

	// Check which parents are missing.
	var missingParents []string
	for _, entry := range msg.GetParents().GetEntries() {
		pk := refKey(entry.GetParent())
		if _, ok := s.vertices[pk]; !ok {
			missingParents = append(missingParents, pk)
		}
	}

	if len(missingParents) == 0 {
		if ok, _ := s.verifyParentTable(msg); !ok {
			return AddResult{Ref: ref}, fmt.Errorf("invalid parent table")
		}
		s.incorporate(key, ref, msg)
		return AddResult{Ref: ref, IsNew: true}, nil
	}

	// Buffer as pending.
	s.pending[key] = msg
	s.missing[key] = len(missingParents)
	for _, pk := range missingParents {
		if s.blockedBy[pk] == nil {
			s.blockedBy[pk] = make(map[string]struct{})
		}
		s.blockedBy[pk][key] = struct{}{}
		s.wanted[pk] = struct{}{}
	}

	// Compute transitive missing ancestors for reactive resolution.
	missingAncestors := s.transitiveMissing(msg)

	return AddResult{Ref: ref, IsNew: true, IsPending: true, MissingAncestors: missingAncestors}, nil
}

// incorporate adds a message to the incorporated store and cascades to
// unblock any pending messages. Caller must hold s.mu.
func (s *Store) incorporate(key string, ref *MessageRef, msg *Message) {
	type entry struct {
		key string
		ref *MessageRef
		msg *Message
	}
	queue := []entry{{key, ref, msg}}

	for len(queue) > 0 {
		e := queue[0]
		queue = queue[1:]

		// Resolve parent pointers.
		var parents []*vertex
		for _, pe := range e.msg.GetParents().GetEntries() {
			pk := refKey(pe.GetParent())
			if pn, ok := s.vertices[pk]; ok {
				parents = append(parents, pn)
			}
		}

		v := &vertex{
			msg:     e.msg,
			ref:     e.ref,
			key:     e.key,
			parents: parents,
		}
		s.vertices[e.key] = v

		// Update children pointers and frontier.
		for _, pn := range parents {
			pn.children = append(pn.children, v)
			delete(s.frontier, pn)
		}

		// This vertex is on the frontier (no children yet).
		s.frontier[v] = struct{}{}

		// Remove from wanted if it was there.
		delete(s.wanted, e.key)

		// Unblock pending messages waiting on this one.
		if waiters, ok := s.blockedBy[e.key]; ok {
			for waiterKey := range waiters {
				s.missing[waiterKey]--
				if s.missing[waiterKey] == 0 {
					delete(s.missing, waiterKey)
					waiterMsg := s.pending[waiterKey]
					delete(s.pending, waiterKey)
					if ok, _ := s.verifyParentTable(waiterMsg); !ok {
						continue
					}
					waiterRef, _ := HashMessage(waiterMsg)
					queue = append(queue, entry{waiterKey, waiterRef, waiterMsg})
				}
			}
			delete(s.blockedBy, e.key)
		}
	}
}

// transitiveMissing walks backward from msg's parents to find all refs that
// are neither incorporated nor pending — these are the refs we need from peers.
// Caller must hold s.mu.
func (s *Store) transitiveMissing(msg *Message) []*MessageRef {
	visited := make(map[string]struct{})
	var result []*MessageRef

	// Seed with the message's direct parents.
	queue := make([]string, 0, len(msg.GetParents().GetEntries()))
	for _, entry := range msg.GetParents().GetEntries() {
		pk := refKey(entry.GetParent())
		if _, ok := visited[pk]; ok {
			continue
		}
		visited[pk] = struct{}{}
		queue = append(queue, pk)
	}

	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]

		if _, ok := s.vertices[k]; ok {
			// Incorporated — stop, we have it.
			continue
		}

		if pmsg, ok := s.pending[k]; ok {
			// Pending — present but blocked. Walk into its parents.
			for _, entry := range pmsg.GetParents().GetEntries() {
				pk := refKey(entry.GetParent())
				if _, ok := visited[pk]; ok {
					continue
				}
				visited[pk] = struct{}{}
				queue = append(queue, pk)
			}
			continue
		}

		// Neither incorporated nor pending — we need this ref.
		b, _ := hex.DecodeString(k)
		result = append(result, &MessageRef{Sha256V1: b})
	}

	return result
}

// CreateMessage builds a parent table from the store's frontier, constructs
// a new signed message, adds it to the store, and returns the message and
// its ref.
func (s *Store) CreateMessage(id *Identity) (*Message, *MessageRef, error) {
	pt := s.BuildParentTable()
	msg, err := NewMessage(id, pt)
	if err != nil {
		return nil, nil, err
	}
	result, err := s.Add(msg)
	if err != nil {
		return nil, nil, err
	}
	return msg, result.Ref, nil
}

// Get returns an incorporated message by ref.
func (s *Store) Get(ref *MessageRef) (*Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.vertices[refKey(ref)]
	if !ok {
		return nil, false
	}
	return n.msg, ok
}

// Children returns the refs of all children of the given message.
func (s *Store) Children(ref *MessageRef) []*MessageRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.vertices[refKey(ref)]
	if !ok {
		return nil
	}
	out := make([]*MessageRef, len(n.children))
	for i, child := range n.children {
		out[i] = child.ref
	}
	return out
}

// Frontier returns the refs of all messages with no children.
func (s *Store) Frontier() []*MessageRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*MessageRef, 0, len(s.frontier))
	for n := range s.frontier {
		out = append(out, n.ref)
	}
	return out
}

// Wanted returns the set of message refs that are needed but not yet received.
func (s *Store) Wanted() []*MessageRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*MessageRef, 0, len(s.wanted))
	for k := range s.wanted {
		b, _ := hex.DecodeString(k)
		out = append(out, &MessageRef{Sha256V1: b})
	}
	return out
}

const maxResolve = 5000

// Resolve walks backward from the wanted refs through incorporated and
// pending messages, stopping at refs in the frontier set or when the walk
// cap is reached. Returns the collected messages (no particular order).
// The caller streams these to the requesting node.
func (s *Store) Resolve(wanted []*MessageRef, frontier []*MessageRef) []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build frontier lookup set.
	stop := make(map[string]struct{}, len(frontier))
	for _, ref := range frontier {
		stop[refKey(ref)] = struct{}{}
	}

	visited := make(map[string]struct{})
	var result []*Message

	// BFS backward from wanted.
	queue := make([]string, 0, len(wanted))
	for _, ref := range wanted {
		k := refKey(ref)
		if _, ok := visited[k]; ok {
			continue
		}
		if _, ok := stop[k]; ok {
			continue
		}
		visited[k] = struct{}{}
		queue = append(queue, k)
	}

	for len(queue) > 0 && len(result) < maxResolve {
		k := queue[0]
		queue = queue[1:]

		// Check incorporated vertices first, then pending.
		var msg *Message
		if n, ok := s.vertices[k]; ok {
			msg = n.msg
		} else if pm, ok := s.pending[k]; ok {
			msg = pm
		}
		if msg == nil {
			continue
		}
		result = append(result, msg)

		for _, entry := range msg.GetParents().GetEntries() {
			pk := refKey(entry.GetParent())
			if _, ok := visited[pk]; ok {
				continue
			}
			if _, ok := stop[pk]; ok {
				continue
			}
			visited[pk] = struct{}{}
			queue = append(queue, pk)
		}
	}

	return result
}

// Len returns the number of incorporated messages.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.vertices)
}

// PendingLen returns the number of messages waiting for parents.
func (s *Store) PendingLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pending)
}
