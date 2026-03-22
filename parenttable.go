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

// buildParentTableFromCandidates runs the greedy parent selection algorithm
// on the given candidates. It selects parents by highest additional
// contribution using interleaved backward BFS to compute overlaps.
// Caller must hold s.mu for reading.
func (s *Store) buildParentTableFromCandidates(candidates []candidate) *ParentTable {
	if len(candidates) == 0 {
		return &ParentTable{}
	}

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

// BuildParentTable constructs a parent table from the store's frontier messages.
func (s *Store) BuildParentTable() *ParentTable {
	s.mu.RLock()
	defer s.mu.RUnlock()

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

	return s.buildParentTableFromCandidates(candidates)
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

			// Skip if already visited by B
			if _, ok := incremental.visited[bk]; ok {
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

// VerifyParentTable checks a message's parent table using the simultaneous
// multi-walk approach. All parents are walked concurrently with priority-based
// attribution. P_1 is verified O(1) and branch walks share a single forward
// verification pass.
func (s *Store) VerifyParentTable(msg *Message) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := msg.GetParents().GetEntries()
	if len(entries) == 0 {
		return true, nil
	}

	// Verify ordering and tiebreaking.
	for i := range entries {
		if i > 0 && entries[i].GetMessageCount() > entries[i-1].GetMessageCount() {
			return false, nil
		}
		if i > 0 && entries[i].GetMessageCount() == entries[i-1].GetMessageCount() {
			prevMsg, ok := s.messages[refKey(entries[i-1].GetParent())]
			if !ok {
				return false, nil
			}
			curMsg, ok := s.messages[refKey(entries[i].GetParent())]
			if !ok {
				return false, nil
			}
			if !validTiebreak(prevMsg, entries[i-1].GetParent(), curMsg, entries[i].GetParent()) {
				return false, nil
			}
		}
	}

	// Verify P_1 count in O(1).
	p1Key := refKey(entries[0].GetParent())
	p1Msg, ok := s.messages[p1Key]
	if !ok {
		return false, nil
	}
	if Total(p1Msg) != entries[0].GetMessageCount() {
		return false, nil
	}

	if len(entries) == 1 {
		return true, nil
	}

	// Initialize per-parent walk states. Index 0 = P_1 (trunk, highest priority).
	type parentWalk struct {
		key     string
		queue   []string
		visited map[string]struct{}
		counter uint64
		claimed uint64
	}
	walks := make([]*parentWalk, len(entries))
	for i, entry := range entries {
		pk := refKey(entry.GetParent())
		walks[i] = &parentWalk{
			key:     pk,
			queue:   []string{pk},
			visited: make(map[string]struct{}),
			claimed: entry.GetMessageCount(),
		}
	}

	// Phase 1: Simultaneous backward walks with priority-based attribution.
	for {
		branchesActive := false
		for i := 1; i < len(walks); i++ {
			if len(walks[i].queue) > 0 {
				branchesActive = true
				break
			}
		}
		if !branchesActive {
			break
		}

		expandIdx := -1
		expandSize := uint64(0)
		for i, w := range walks {
			if len(w.queue) == 0 {
				continue
			}
			sz := uint64(len(w.visited))
			if expandIdx == -1 || sz < expandSize {
				expandIdx = i
				expandSize = sz
			}
		}
		if expandIdx == -1 {
			break
		}

		w := walks[expandIdx]
		mk := w.queue[0]
		w.queue = w.queue[1:]

		if _, ok := w.visited[mk]; ok {
			continue
		}

		claimedByHigher := false
		for j := 0; j < expandIdx; j++ {
			if _, ok := walks[j].visited[mk]; ok {
				claimedByHigher = true
				break
			}
		}
		if claimedByHigher {
			continue
		}

		w.visited[mk] = struct{}{}
		w.counter++

		for k := expandIdx + 1; k < len(walks); k++ {
			if _, ok := walks[k].visited[mk]; ok {
				delete(walks[k].visited, mk)
				walks[k].counter--
			}
		}

		if msg, ok := s.messages[mk]; ok {
			for _, entry := range msg.GetParents().GetEntries() {
				pk := refKey(entry.GetParent())
				w.queue = append(w.queue, pk)
			}
		}
	}

	// Phase 2: Forward verification on all branch parents simultaneously.
	type fwdRoot struct {
		parentIdx int
		key       string
	}
	type fwdOrigin struct {
		roots []fwdRoot
	}

	fwdVisited := make(map[string]*fwdOrigin)
	var fwdQueue []string

	for i := 1; i < len(walks); i++ {
		for mk := range walks[i].visited {
			root := fwdRoot{parentIdx: i, key: mk}
			if existing, ok := fwdVisited[mk]; ok {
				existing.roots = append(existing.roots, root)
			} else {
				fwdVisited[mk] = &fwdOrigin{roots: []fwdRoot{root}}
				fwdQueue = append(fwdQueue, mk)
			}
		}
	}

	for len(fwdQueue) > 0 {
		k := fwdQueue[0]
		fwdQueue = fwdQueue[1:]
		o := fwdVisited[k]

		for _, root := range o.roots {
			for j := 0; j < root.parentIdx; j++ {
				if _, ok := walks[j].visited[k]; ok {
					if _, ok := walks[root.parentIdx].visited[root.key]; ok {
						delete(walks[root.parentIdx].visited, root.key)
						walks[root.parentIdx].counter--
					}
					break
				}
			}
		}

		kids, ok := s.children[k]
		if !ok {
			continue
		}
		for childKey := range kids {
			if existing, ok := fwdVisited[childKey]; ok {
				existing.roots = append(existing.roots, o.roots...)
			} else {
				newO := &fwdOrigin{roots: make([]fwdRoot, len(o.roots))}
				copy(newO.roots, o.roots)
				fwdVisited[childKey] = newO
				fwdQueue = append(fwdQueue, childKey)
			}
		}
	}

	// Verify counts.
	for i := 1; i < len(walks); i++ {
		if walks[i].counter != walks[i].claimed {
			return false, nil
		}
	}

	return true, nil
}

// VerifyParentTableByConstruction verifies a parent table by reconstructing
// it from the same set of parents using the greedy construction algorithm,
// then comparing the result entry-by-entry.
func (s *Store) VerifyParentTableByConstruction(msg *Message) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := msg.GetParents().GetEntries()
	if len(entries) == 0 {
		return true, nil
	}

	// Build candidate set from the parent table's parents.
	candidates := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		pk := refKey(entry.GetParent())
		parentMsg, ok := s.messages[pk]
		if !ok {
			return false, nil
		}
		candidates = append(candidates, candidate{
			key: pk,
			ref: entry.GetParent(),
			msg: parentMsg,
		})
	}

	reconstructed := s.buildParentTableFromCandidates(candidates)

	// Compare reconstructed vs original.
	rEntries := reconstructed.GetEntries()
	if len(rEntries) != len(entries) {
		return false, nil
	}
	for i := range entries {
		if refKey(entries[i].GetParent()) != refKey(rEntries[i].GetParent()) {
			return false, nil
		}
		if entries[i].GetMessageCount() != rEntries[i].GetMessageCount() {
			return false, nil
		}
	}
	return true, nil
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
