package braid

import (
	"bytes"
	"sort"
)

type candidate struct {
	key string
	ref *MessageRef
	msg *Message
}

// walkState tracks the progress of a backward BFS walk through the DAG.
// It holds the set of visited messages and the queue of messages whose
// parents have not yet been explored.
type walkState struct {
	visited map[string]struct{}
	queue   []string
}

func newWalkState() *walkState {
	return &walkState{visited: make(map[string]struct{})}
}

// merge incorporates another walk state into this one. The other's visited
// set is unioned in, and queue entries not already in this state's visited
// set are appended to this state's queue.
func (a *walkState) merge(b *walkState) {
	for k := range b.visited {
		a.visited[k] = struct{}{}
	}
	for _, k := range b.queue {
		if _, ok := a.visited[k]; !ok {
			a.queue = append(a.queue, k)
		}
	}
}

// seed adds a root message to the walk state if not already visited.
func (a *walkState) seed(key string) {
	if _, ok := a.visited[key]; !ok {
		a.queue = append(a.queue, key)
	}
}

// Total returns the total number of messages backward-reachable from msg
// (including msg itself). This is O(1), computed from the parent table.
func Total(msg *Message) uint64 {
	var sum uint64
	for _, entry := range msg.GetParents().GetEntries() {
		sum += entry.GetMessageCount()
	}
	return sum + 1
}

// BuildParentTable constructs a parent table from the given frontier messages.
// It greedily selects parents by highest additional contribution using
// interleaved backward BFS to compute overlaps.
func (s *Store) BuildParentTable() *ParentTable {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.frontier) == 0 {
		return &ParentTable{}
	}

	// Collect frontier messages as candidates.
	candidates := make([]candidate, 0, len(s.frontier))
	for k := range s.frontier {
		msg := s.messages[k]
		b, _ := hexDecode(k)
		candidates = append(candidates, candidate{
			key: k,
			ref: &MessageRef{Sha256V1: b},
			msg: msg,
		})
	}

	// Accumulator: tracks the covered region across rounds.
	accum := newWalkState()

	var entries []*ParentTable_Entry

	// Round 1: select first parent using Total() (O(1) per candidate).
	{
		bestIdx := -1
		var bestCount uint64
		for i, c := range candidates {
			count := Total(c.msg)
			if count > bestCount || (count == bestCount && betterTiebreak(c.msg, c.ref, bestIdx, candidates)) {
				bestCount = count
				bestIdx = i
			}
		}

		if bestIdx == -1 || bestCount == 0 {
			return &ParentTable{}
		}

		best := candidates[bestIdx]
		mc := bestCount
		entries = append(entries, &ParentTable_Entry{
			Parent:       best.ref,
			MessageCount: &mc,
		})

		// Seed accumulator with the first parent.
		accum.seed(best.key)

		candidates[bestIdx] = candidates[len(candidates)-1]
		candidates = candidates[:len(candidates)-1]
	}

	// Round 2+: select subsequent parents using interleaved BFS.
	for len(candidates) > 0 {
		bestIdx := -1
		var bestCount uint64
		var bestIncremental *walkState

		for i, c := range candidates {
			count, incremental := s.additional(c.key, accum)
			if count > bestCount || (count == bestCount && betterTiebreak(c.msg, c.ref, bestIdx, candidates)) {
				bestCount = count
				bestIdx = i
				bestIncremental = incremental
			}
		}

		if bestIdx == -1 || bestCount == 0 {
			break
		}

		best := candidates[bestIdx]
		mc := bestCount
		entries = append(entries, &ParentTable_Entry{
			Parent:       best.ref,
			MessageCount: &mc,
		})

		accum.merge(bestIncremental)

		candidates[bestIdx] = candidates[len(candidates)-1]
		candidates = candidates[:len(candidates)-1]
	}

	return &ParentTable{Entries: entries}
}

// additional computes |reachable(key) \ accum.visited| using interleaved
// backward BFS with pace control and forward verification.
//
// The accumulator's walk state is expanded in place on the A side.
// The candidate's walk state is returned as the incremental result.
//
// Phase 1: Interleaved backward walk. A expands the accumulator, B walks
// backward from the candidate. Pace control: expand A when
// |accum.visited| <= |incremental.visited|, expand B otherwise.
// Phase 1 ends when B's queue is empty.
//
// Phase 2: Forward verification on incremental.visited to remove false
// positives.
//
// Caller must hold s.mu for reading.
func (s *Store) additional(key string, accum *walkState) (uint64, *walkState) {
	incremental := newWalkState()
	incremental.queue = []string{key}

	// Phase 1: Interleaved backward walk.
	for len(incremental.queue) > 0 {
		if len(accum.visited) <= len(incremental.visited) && len(accum.queue) > 0 {
			// Expand A one step.
			ak := accum.queue[0]
			accum.queue = accum.queue[1:]

			// Add as visited, enqueue parents
			accum.visited[ak] = struct{}{}
			if msg, ok := s.messages[ak]; ok {
				for _, entry := range msg.GetParents().GetEntries() {
					pk := refKey(entry.GetParent())
					accum.queue = append(accum.queue, pk)
				}
			}

			// Remove if visited first on B side (not incremental)
			delete(incremental.visited, ak)
		} else {
			// Expand B one step.
			bk := incremental.queue[0]
			incremental.queue = incremental.queue[1:]

			// Skip if visited first on A side (not incremental)
			if _, ok := accum.visited[bk]; ok {
				continue
			}

			// Add as incremental, enqueue parents
			incremental.visited[bk] = struct{}{}
			if msg, ok := s.messages[bk]; ok {
				for _, entry := range msg.GetParents().GetEntries() {
					pk := refKey(entry.GetParent())
					incremental.queue = append(incremental.queue, pk)
				}
			}
		}
	}

	// Phase 2: Forward verification.
	s.forwardVerify(incremental.visited, accum.visited)

	// The remaining queue entries in incremental are parents of verified
	// unique messages — they form the frontier for future merging.
	return uint64(len(incremental.visited)), incremental
}

// forwardVerify removes false positives from incremental by walking forward
// through the children index. A message in incremental that can reach any
// message in visited is a false positive.
func (s *Store) forwardVerify(incremental map[string]struct{}, visited map[string]struct{}) {
	if len(incremental) == 0 {
		return
	}

	// Batch BFS forward from all incremental messages simultaneously.
	type origin struct {
		roots map[string]struct{}
	}

	fwdVisited := make(map[string]*origin)
	var fwdQueue []string

	for k := range incremental {
		o := &origin{roots: map[string]struct{}{k: {}}}
		fwdVisited[k] = o
		fwdQueue = append(fwdQueue, k)
	}

	for len(fwdQueue) > 0 {
		k := fwdQueue[0]
		fwdQueue = fwdQueue[1:]
		o := fwdVisited[k]

		if _, ok := visited[k]; ok {
			for root := range o.roots {
				delete(incremental, root)
			}
			continue
		}

		kids, ok := s.children[k]
		if !ok {
			continue
		}
		for childKey := range kids {
			if existing, ok := fwdVisited[childKey]; ok {
				for root := range o.roots {
					existing.roots[root] = struct{}{}
				}
			} else {
				newO := &origin{roots: make(map[string]struct{}, len(o.roots))}
				for root := range o.roots {
					newO.roots[root] = struct{}{}
				}
				fwdVisited[childKey] = newO
				fwdQueue = append(fwdQueue, childKey)
			}
		}
	}
}

// betterTiebreak returns true if candidate c should be preferred over the
// current best when counts are equal. Tiebreak: later timestamp first,
// then lexicographic on ref.
func betterTiebreak(msg *Message, ref *MessageRef, bestIdx int, candidates []candidate) bool {
	if bestIdx == -1 {
		return true
	}
	bestMsg := candidates[bestIdx].msg
	bestRef := candidates[bestIdx].ref

	cTS := msg.GetTimestamp()
	bTS := bestMsg.GetTimestamp()
	if cTS.GetSeconds() != bTS.GetSeconds() {
		return cTS.GetSeconds() > bTS.GetSeconds()
	}
	if cTS.GetNanos() != bTS.GetNanos() {
		return cTS.GetNanos() > bTS.GetNanos()
	}

	return bytes.Compare(ref.GetSha256V1(), bestRef.GetSha256V1()) < 0
}

func hexDecode(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(b); i++ {
		b[i] = unhex(s[2*i])<<4 | unhex(s[2*i+1])
	}
	return b, nil
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// VerifyParentTable checks that a message's parent table has correct counts
// and ordering. The store must contain all referenced parent messages.
func (s *Store) VerifyParentTable(msg *Message) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := msg.GetParents().GetEntries()
	if len(entries) == 0 {
		return true, nil
	}

	accum := newWalkState()

	for i, entry := range entries {
		pk := refKey(entry.GetParent())

		// Verify count.
		count, incremental := s.additional(pk, accum)
		if count != entry.GetMessageCount() {
			return false, nil
		}

		// Verify ordering: count must be <= previous count.
		if i > 0 && entry.GetMessageCount() > entries[i-1].GetMessageCount() {
			return false, nil
		}

		// Verify tiebreaking if counts are equal.
		if i > 0 && entry.GetMessageCount() == entries[i-1].GetMessageCount() {
			prevMsg, ok := s.messages[refKey(entries[i-1].GetParent())]
			if !ok {
				return false, nil
			}
			curMsg, ok := s.messages[pk]
			if !ok {
				return false, nil
			}
			if !validTiebreak(prevMsg, entries[i-1].GetParent(), curMsg, entry.GetParent()) {
				return false, nil
			}
		}

		// Merge incremental and seed for next entry.
		accum.merge(incremental)
		accum.seed(pk)
	}

	return true, nil
}

// validTiebreak checks that prev should come before cur in tiebreak order.
// Later timestamp first, then lexicographic on ref.
func validTiebreak(prevMsg *Message, prevRef *MessageRef, curMsg *Message, curRef *MessageRef) bool {
	pTS := prevMsg.GetTimestamp()
	cTS := curMsg.GetTimestamp()
	if pTS.GetSeconds() != cTS.GetSeconds() {
		return pTS.GetSeconds() > cTS.GetSeconds()
	}
	if pTS.GetNanos() != cTS.GetNanos() {
		return pTS.GetNanos() > cTS.GetNanos()
	}
	return bytes.Compare(prevRef.GetSha256V1(), curRef.GetSha256V1()) <= 0
}

// SortParentTable sorts a parent table by message_count descending, with
// tiebreaking by timestamp (later first) then lexicographic on ref.
func SortParentTable(entries []*ParentTable_Entry, store *Store) {
	sort.SliceStable(entries, func(i, j int) bool {
		ci := entries[i].GetMessageCount()
		cj := entries[j].GetMessageCount()
		if ci != cj {
			return ci > cj
		}
		mi, _ := store.Get(entries[i].GetParent())
		mj, _ := store.Get(entries[j].GetParent())
		if mi != nil && mj != nil {
			iTS := mi.GetTimestamp()
			jTS := mj.GetTimestamp()
			if iTS.GetSeconds() != jTS.GetSeconds() {
				return iTS.GetSeconds() > jTS.GetSeconds()
			}
			if iTS.GetNanos() != jTS.GetNanos() {
				return iTS.GetNanos() > jTS.GetNanos()
			}
		}
		return bytes.Compare(entries[i].GetParent().GetSha256V1(), entries[j].GetParent().GetSha256V1()) < 0
	})
}
