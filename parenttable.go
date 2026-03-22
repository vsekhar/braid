package braid

import (
	"bytes"
)

// walkState tracks a backward BFS walk using direct vertex pointers.
type walkState struct {
	visited map[*vertex]struct{}
	queue   []*vertex
}

func newWalkState() *walkState {
	return &walkState{visited: make(map[*vertex]struct{})}
}

func (a *walkState) merge(b *walkState) {
	for n := range b.visited {
		a.visited[n] = struct{}{}
	}
	for _, n := range b.queue {
		if _, ok := a.visited[n]; !ok {
			a.queue = append(a.queue, n)
		}
	}
}

func (a *walkState) seed(n *vertex) {
	if _, ok := a.visited[n]; !ok {
		a.queue = append(a.queue, n)
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

// buildTableFromParents runs the greedy algorithm on the given parents
// and orders them by highest additional contribution using interleaved
// backward BFS to compute overlaps.
// Caller must hold s.mu for reading.
func (s *Store) buildTableFromParents(parents []*vertex) *ParentTable {
	if len(parents) == 0 {
		return &ParentTable{}
	}

	accum := newWalkState()
	var entries []*ParentTable_Entry

	// Round 1: select first parent using Total() (O(1) per parent).
	{
		bestIdx := -1
		var bestCount uint64
		for i, c := range parents {
			count := Total(c.msg)
			if count > bestCount || (count == bestCount && betterTiebreak(c, bestIdx, parents)) {
				bestCount = count
				bestIdx = i
			}
		}

		if bestIdx == -1 || bestCount == 0 {
			return &ParentTable{}
		}

		best := parents[bestIdx]
		mc := bestCount
		entries = append(entries, &ParentTable_Entry{
			Parent:       best.ref,
			MessageCount: &mc,
		})

		accum.seed(best)

		parents[bestIdx] = parents[len(parents)-1]
		parents = parents[:len(parents)-1]
	}

	// Round 2+: select subsequent parents using interleaved BFS.
	for len(parents) > 0 {
		bestIdx := -1
		var bestCount uint64
		var bestIncremental *walkState

		for i, c := range parents {
			count, incremental := s.additional(c, accum)
			if count > bestCount || (count == bestCount && betterTiebreak(c, bestIdx, parents)) {
				bestCount = count
				bestIdx = i
				bestIncremental = incremental
			}
		}

		if bestIdx == -1 || bestCount == 0 {
			break
		}

		best := parents[bestIdx]
		mc := bestCount
		entries = append(entries, &ParentTable_Entry{
			Parent:       best.ref,
			MessageCount: &mc,
		})

		accum.merge(bestIncremental)

		parents[bestIdx] = parents[len(parents)-1]
		parents = parents[:len(parents)-1]
	}

	return &ParentTable{Entries: entries}
}

// BuildParentTable constructs a parent table from the store's frontier messages.
func (s *Store) BuildParentTable() *ParentTable {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Naively select all messages on frontier as parents.
	// TODO: smarter selection algo
	parents := make([]*vertex, 0, len(s.frontier))
	for n := range s.frontier {
		parents = append(parents, n)
	}

	return s.buildTableFromParents(parents)
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
func (s *Store) additional(target *vertex, accum *walkState) (uint64, *walkState) {
	incremental := newWalkState()
	incremental.queue = []*vertex{target}

	// Phase 1: Interleaved backward walk.
	for len(incremental.queue) > 0 {
		if len(accum.visited) <= len(incremental.visited) && len(accum.queue) > 0 {
			// Expand A one step.
			ak := accum.queue[0]
			accum.queue = accum.queue[1:]

			if _, ok := accum.visited[ak]; ok {
				continue
			}

			// Add as visited, enqueue unvisited parents
			accum.visited[ak] = struct{}{}
			for _, pk := range ak.parents {
				if _, ok := accum.visited[pk]; !ok {
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

			// Add as incremental, enqueue parents not already seen
			incremental.visited[bk] = struct{}{}
			for _, pk := range bk.parents {
				if _, ok := incremental.visited[pk]; !ok {
					if _, ok := accum.visited[pk]; !ok {
						incremental.queue = append(incremental.queue, pk)
					}
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
func (s *Store) forwardVerify(incremental map[*vertex]struct{}, visited map[*vertex]struct{}) {
	if len(incremental) == 0 {
		return
	}

	// Batch BFS forward from all incremental messages simultaneously.
	type origin struct {
		roots map[*vertex]struct{}
	}

	fwdVisited := make(map[*vertex]*origin)
	var fwdQueue []*vertex

	for n := range incremental {
		o := &origin{roots: map[*vertex]struct{}{n: {}}}
		fwdVisited[n] = o
		fwdQueue = append(fwdQueue, n)
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

		for _, child := range k.children {
			if existing, ok := fwdVisited[child]; ok {
				// Merge roots. Re-enqueue if we added new roots so they
				// propagate forward through already-visited children.
				oldLen := len(existing.roots)
				for root := range o.roots {
					existing.roots[root] = struct{}{}
				}
				if len(existing.roots) > oldLen {
					fwdQueue = append(fwdQueue, child)
				}
			} else {
				newO := &origin{roots: make(map[*vertex]struct{}, len(o.roots))}
				for root := range o.roots {
					newO.roots[root] = struct{}{}
				}
				fwdVisited[child] = newO
				fwdQueue = append(fwdQueue, child)
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
	return s.verifyParentTable(msg)
}

// verifyParentTable is the lock-free implementation of VerifyParentTable.
// Caller must hold s.mu for reading.
func (s *Store) verifyParentTable(msg *Message) (bool, error) {
	entries := msg.GetParents().GetEntries()
	if len(entries) == 0 {
		return true, nil
	}

	// Resolve all parent refs to vertices and verify ordering/tiebreaking.
	parents := make([]*vertex, len(entries))
	for i, entry := range entries {
		n, ok := s.vertices[refKey(entry.GetParent())]
		if !ok {
			return false, nil
		}
		parents[i] = n

		if i > 0 && entries[i].GetMessageCount() > entries[i-1].GetMessageCount() {
			return false, nil
		}
		if i > 0 && entries[i].GetMessageCount() == entries[i-1].GetMessageCount() {
			if !validTiebreak(parents[i-1].msg, parents[i-1].ref,
				parents[i].msg, parents[i].ref) {
				return false, nil
			}
		}
	}

	// Verify P_1 count in O(1).
	if Total(parents[0].msg) != entries[0].GetMessageCount() {
		return false, nil
	}

	if len(entries) == 1 {
		return true, nil
	}

	// Initialize per-parent walk states. Index 0 = P_1 (trunk, highest priority).
	type parentWalk struct {
		n       *vertex
		queue   []*vertex
		visited map[*vertex]struct{}
		counter uint64
		claimed uint64
	}
	walks := make([]*parentWalk, len(entries))
	for i, entry := range entries {
		walks[i] = &parentWalk{
			n:       parents[i],
			queue:   []*vertex{parents[i]},
			visited: make(map[*vertex]struct{}),
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

		for _, pk := range mk.parents {
			if _, ok := w.visited[pk]; !ok {
				w.queue = append(w.queue, pk)
			}
		}
	}

	// Phase 2: Forward verification on all branch parents.
	// Build a cumulative "trusted" set from higher-priority walks.
	// For each branch, forward-verify its visited set against the trusted set,
	// then add its verified visited set to the trusted set for the next branch.
	trusted := make(map[*vertex]struct{})
	for k := range walks[0].visited {
		trusted[k] = struct{}{}
	}
	for i := 1; i < len(walks); i++ {
		s.forwardVerify(walks[i].visited, trusted)
		walks[i].counter = uint64(len(walks[i].visited))
		for k := range walks[i].visited {
			trusted[k] = struct{}{}
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

	parents := make([]*vertex, 0, len(entries))
	for _, entry := range entries {
		pk := refKey(entry.GetParent())
		n, ok := s.vertices[pk]
		if !ok {
			return false, nil
		}
		parents = append(parents, n)
	}

	reconstructed := s.buildTableFromParents(parents)

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
func betterTiebreak(c *vertex, bestIdx int, candidates []*vertex) bool {
	if bestIdx == -1 {
		return true
	}
	bestVtx := candidates[bestIdx]
	cTS := c.msg.GetTimestamp()
	bTS := bestVtx.msg.GetTimestamp()
	if cTS.GetSeconds() != bTS.GetSeconds() {
		return cTS.GetSeconds() > bTS.GetSeconds()
	}
	if cTS.GetNanos() != bTS.GetNanos() {
		return cTS.GetNanos() > bTS.GetNanos()
	}

	return bytes.Compare(c.ref.GetSha256V1(), bestVtx.ref.GetSha256V1()) < 0
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
