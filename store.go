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

// Store holds the message DAG, including incorporated messages, pending
// messages awaiting parents, and the frontier (messages with no children).
type Store struct {
	mu sync.RWMutex

	// Incorporated messages.
	messages map[string]*Message    // refKey → message
	children map[string]map[string]struct{} // refKey → set of child refKeys
	frontier map[string]struct{}    // refKeys of messages with no children

	// Pending messages awaiting missing parents.
	pending   map[string]*Message            // refKey → message
	blockedBy map[string]map[string]struct{} // missing parent refKey → set of pending refKeys waiting on it
	missing   map[string]int                 // pending refKey → count of missing parents
	wanted    map[string]struct{}            // refKeys we need to request from peers
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		messages:  make(map[string]*Message),
		children:  make(map[string]map[string]struct{}),
		frontier:  make(map[string]struct{}),
		pending:   make(map[string]*Message),
		blockedBy: make(map[string]map[string]struct{}),
		missing:   make(map[string]int),
		wanted:    make(map[string]struct{}),
	}
}

// Add attempts to incorporate a message into the store. If parents are
// missing, the message is buffered as pending and the missing parent refs
// are added to the wanted set. Returns the message's ref and whether it
// was newly added (not a duplicate).
func (s *Store) Add(msg *Message) (*MessageRef, bool, error) {
	ref, err := HashMessage(msg)
	if err != nil {
		return nil, false, fmt.Errorf("hashing message: %w", err)
	}
	key := refKey(ref)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip duplicates.
	if _, ok := s.messages[key]; ok {
		return ref, false, nil
	}
	if _, ok := s.pending[key]; ok {
		return ref, false, nil
	}

	// Check which parents are missing.
	var missingParents []string
	for _, entry := range msg.GetParents().GetEntries() {
		pk := refKey(entry.GetParent())
		if _, ok := s.messages[pk]; !ok {
			missingParents = append(missingParents, pk)
		}
	}

	if len(missingParents) == 0 {
		s.incorporate(key, msg)
		return ref, true, nil
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
	return ref, true, nil
}

// incorporate adds a message to the incorporated store and cascades to
// unblock any pending messages. Caller must hold s.mu.
func (s *Store) incorporate(key string, msg *Message) {
	// Use a queue to avoid deep recursion.
	type entry struct {
		key string
		msg *Message
	}
	queue := []entry{{key, msg}}

	for len(queue) > 0 {
		e := queue[0]
		queue = queue[1:]

		s.messages[e.key] = e.msg

		// Update children index and frontier for each parent.
		for _, pe := range e.msg.GetParents().GetEntries() {
			pk := refKey(pe.GetParent())
			if s.children[pk] == nil {
				s.children[pk] = make(map[string]struct{})
			}
			s.children[pk][e.key] = struct{}{}
			// Parent is no longer on the frontier.
			delete(s.frontier, pk)
		}

		// This message is on the frontier (no children yet).
		s.frontier[e.key] = struct{}{}

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
					queue = append(queue, entry{waiterKey, waiterMsg})
				}
			}
			delete(s.blockedBy, e.key)
		}
	}
}

// Get returns an incorporated message by ref.
func (s *Store) Get(ref *MessageRef) (*Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msg, ok := s.messages[refKey(ref)]
	return msg, ok
}

// Children returns the refs of all children of the given message.
func (s *Store) Children(ref *MessageRef) []*MessageRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	kids, ok := s.children[refKey(ref)]
	if !ok {
		return nil
	}
	out := make([]*MessageRef, 0, len(kids))
	for k := range kids {
		b, _ := hex.DecodeString(k)
		out = append(out, &MessageRef{Sha256V1: b})
	}
	return out
}

// Frontier returns the refs of all messages with no children.
func (s *Store) Frontier() []*MessageRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*MessageRef, 0, len(s.frontier))
	for k := range s.frontier {
		b, _ := hex.DecodeString(k)
		out = append(out, &MessageRef{Sha256V1: b})
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

// Len returns the number of incorporated messages.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

// PendingLen returns the number of messages waiting for parents.
func (s *Store) PendingLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pending)
}
