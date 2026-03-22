package braid

import (
	"testing"
)

// wideDAGResult holds the result of building a wide DAG with many frontier
// messages, suitable for testing parent table construction and verification
// with more than 2 entries.
type wideDAGResult struct {
	store         *Store
	identity      *Identity
	genesis       *MessageRef
	trunkTip      *MessageRef
	frontierCount int
	totalMessages int
}

// buildWideDAG builds a DAG with a tree-of-branches structure and
// cross-linking diamond patterns. The shape:
//
//	                     (genesis)
//	                        |
//	                   [trunk: 100 msgs]
//	                        |
//	        +-------+-------+-------+-------+
//	        |       |       |       |       |
//	     [br0:30] [br1:25] [br2:20] [br3:15] [br4:10]
//	      / \       / \
//	[sub0a] [sub0b] [sub1a] [sub1b]
//	 :10     :10     :10     :8
//	  \     /
//	[diamond merge]
//	     :5
//	    (tip0)
//	               (tip1a) (tip1b)  (tip2)  (tip3)  (tip4)
//
// Frontier: tip0, tip1a, tip1b, tip2, tip3, tip4 = 6 tips
// Properties: tree branching, sub-branches, diamond pattern, varying depths
func buildWideDAG(t *testing.T) *wideDAGResult {
	t.Helper()
	s := NewStore()
	id := testID(t)
	r := &wideDAGResult{store: s, identity: id}

	// Helper to build a chain of n messages from a given parent.
	buildChain := func(parentRef *MessageRef, parentTotal uint64, n int) (*MessageRef, uint64) {
		ref := parentRef
		total := parentTotal
		for range n {
			_, newRef := addMsg(t, s, id, parentTable(ptEntry(ref, total)))
			total = total + 1
			ref = newRef
		}
		return ref, total
	}

	count := 0

	// Genesis
	_, genesisRef := addMsg(t, s, id, emptyParents())
	r.genesis = genesisRef
	count++

	// Shared trunk: 100 messages
	trunkTip, trunkTotal := buildChain(genesisRef, 1, 100)
	r.trunkTip = trunkTip
	count += 100

	// === Branch 0: 30 messages, then diamond ===
	br0Tip, br0Total := buildChain(trunkTip, trunkTotal, 30)
	count += 30

	// Sub-branch 0a: 10 messages
	sub0a, sub0aTotal := buildChain(br0Tip, br0Total, 10)
	count += 10

	// Sub-branch 0b: 10 messages (parallel)
	sub0b, _ := buildChain(br0Tip, br0Total, 10)
	count += 10

	// Diamond merge: use BuildParentTable-style counting
	// sub0a reaches br0 + trunk + itself = br0Total + 10 = 141
	// sub0b additional = 10 (its unique messages)
	sub0bAdditional := uint64(10)
	_, dm0Ref := addMsg(t, s, id, parentTable(
		ptEntry(sub0a, sub0aTotal),
		ptEntry(sub0b, sub0bAdditional),
	))
	count++
	dm0Total := sub0aTotal + sub0bAdditional + 1

	// Continue 5 messages after diamond merge → tip0
	_, _ = buildChain(dm0Ref, dm0Total, 5)
	count += 5

	// === Branch 1: 25 messages, then two sub-branches (no merge) ===
	br1Tip, br1Total := buildChain(trunkTip, trunkTotal, 25)
	count += 25

	// Sub-branch 1a: 10 messages → tip1a
	_, _ = buildChain(br1Tip, br1Total, 10)
	count += 10

	// Sub-branch 1b: 8 messages → tip1b
	_, _ = buildChain(br1Tip, br1Total, 8)
	count += 8

	// === Branch 2: 20 messages → tip2 ===
	_, _ = buildChain(trunkTip, trunkTotal, 20)
	count += 20

	// === Branch 3: 15 messages → tip3 ===
	_, _ = buildChain(trunkTip, trunkTotal, 15)
	count += 15

	// === Branch 4: 10 messages → tip4 ===
	_, _ = buildChain(trunkTip, trunkTotal, 10)
	count += 10

	r.totalMessages = count
	r.frontierCount = 6 // tip0, tip1a, tip1b, tip2, tip3, tip4

	return r
}

// TestSimpleThreeBranch tests a simple 3-branch DAG to isolate
// the overcounting issue.
//
//	G → trunk(5) → tip
//	              → A(3) = tip_a
//	              → B(2) = tip_b
//	              → C(1) = tip_c
//
// Total messages: 1 + 5 + 3 + 2 + 1 = 12
// tip_a Total = 9, tip_b Total = 8, tip_c Total = 7
// P1 = tip_a (9), additional(tip_b|tip_a) = 2, additional(tip_c|{tip_a,tip_b}) = 1
func TestSimpleThreeBranch(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, gRef := addMsg(t, s, id, emptyParents())
	trunkTip, trunkTotal := gRef, uint64(1)
	for range 5 {
		_, ref := addMsg(t, s, id, parentTable(ptEntry(trunkTip, trunkTotal)))
		trunkTotal++
		trunkTip = ref
	}
	// trunkTotal = 6

	// Three branches
	tipA := trunkTip
	totalA := trunkTotal
	for range 3 {
		_, ref := addMsg(t, s, id, parentTable(ptEntry(tipA, totalA)))
		totalA++
		tipA = ref
	}
	// totalA = 9

	tipB := trunkTip
	totalB := trunkTotal
	for range 2 {
		_, ref := addMsg(t, s, id, parentTable(ptEntry(tipB, totalB)))
		totalB++
		tipB = ref
	}
	// totalB = 8

	tipC := trunkTip
	totalC := trunkTotal
	for range 1 {
		_, ref := addMsg(t, s, id, parentTable(ptEntry(tipC, totalC)))
		totalC++
		tipC = ref
	}
	// totalC = 7

	_ = tipA
	_ = tipB
	_ = tipC

	frontier := s.Frontier()
	t.Logf("Messages: %d, Frontier: %d", s.Len(), len(frontier))

	pt := s.BuildParentTable()
	var sum uint64
	for i, entry := range pt.Entries {
		t.Logf("entry %d: count=%d", i, entry.GetMessageCount())
		sum += entry.GetMessageCount()
	}
	t.Logf("sum=%d, total=%d", sum, s.Len())

	if int(sum) != s.Len() {
		t.Errorf("sum %d != store.Len %d", sum, s.Len())
	}

	// Expected: P1=9 (tip_a), P2=2 (tip_b unique), P3=1 (tip_c unique)
	if len(pt.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(pt.Entries))
	}
	// Debug: brute force reachable sets
	reachable := func(ref *MessageRef) map[string]struct{} {
		set := make(map[string]struct{})
		queue := []string{refKey(ref)}
		for len(queue) > 0 {
			k := queue[0]
			queue = queue[1:]
			if _, ok := set[k]; ok {
				continue
			}
			set[k] = struct{}{}
			if msg, ok := s.Get(&MessageRef{Sha256V1: func() []byte { b, _ := hexDecode(k); return b }()}); ok {
				for _, e := range msg.GetParents().GetEntries() {
					queue = append(queue, refKey(e.GetParent()))
				}
			}
		}
		return set
	}

	// Get the frontier refs
	fr := s.Frontier()
	for i, f := range fr {
		msg, _ := s.Get(f)
		t.Logf("frontier %d: ref=%s total=%d", i, refKey(f)[:8], Total(msg))
	}

	// For entry 0 (the selected first parent), compute brute force additional
	// of entry 1 against entry 0
	if len(pt.Entries) >= 2 {
		r0 := reachable(pt.Entries[0].GetParent())
		r1 := reachable(pt.Entries[1].GetParent())
		additional := 0
		var falsePositives []string
		for k := range r1 {
			if _, ok := r0[k]; !ok {
				additional++
			}
		}
		t.Logf("brute force additional(entry1 | entry0) = %d", additional)
		t.Logf("|reachable(entry0)| = %d, |reachable(entry1)| = %d", len(r0), len(r1))

		// Check which messages in entry1 are false positives
		// (in both r0 and r1 but would be counted by additional)
		for k := range r1 {
			if _, ok := r0[k]; ok {
				// This message is shared — should NOT be in incremental
				falsePositives = append(falsePositives, k)
			}
		}
		t.Logf("shared messages (false positives if in incremental): %d", len(falsePositives))

		// Check children index for trunk tip
		trunkTipKids := s.Children(trunkTip)
		t.Logf("trunk tip children: %d", len(trunkTipKids))
	}

	if pt.Entries[0].GetMessageCount() != 9 {
		t.Errorf("entry 0 count = %d, want 9", pt.Entries[0].GetMessageCount())
	}
	if pt.Entries[1].GetMessageCount() != 2 {
		t.Errorf("entry 1 count = %d, want 2", pt.Entries[1].GetMessageCount())
	}
	if pt.Entries[2].GetMessageCount() != 1 {
		t.Errorf("entry 2 count = %d, want 1", pt.Entries[2].GetMessageCount())
	}
}

func TestWideDAG_Structure(t *testing.T) {
	r := buildWideDAG(t)

	if r.store.Len() != r.totalMessages {
		t.Errorf("store has %d messages, want %d", r.store.Len(), r.totalMessages)
	}

	frontier := r.store.Frontier()
	if len(frontier) != r.frontierCount {
		t.Fatalf("frontier has %d messages, want %d", len(frontier), r.frontierCount)
	}

	if r.store.PendingLen() != 0 {
		t.Errorf("store has %d pending messages, want 0", r.store.PendingLen())
	}

	t.Logf("Wide DAG: %d messages, %d frontier, %d pending",
		r.store.Len(), len(frontier), r.store.PendingLen())
}

func TestWideDAG_BuildParentTable(t *testing.T) {
	r := buildWideDAG(t)

	pt := r.store.BuildParentTable()
	if len(pt.Entries) != r.frontierCount {
		t.Fatalf("got %d entries, want %d", len(pt.Entries), r.frontierCount)
	}

	// Log entries.
	var sum uint64
	for i, entry := range pt.Entries {
		t.Logf("entry %d: count=%d ref=%s",
			i, entry.GetMessageCount(), refKey(entry.GetParent())[:8])
		sum += entry.GetMessageCount()
	}

	// Sum should equal total messages.
	if int(sum) != r.totalMessages {
		t.Errorf("sum of counts %d != totalMessages %d", sum, r.totalMessages)
	}

	// Ordering: descending by count.
	for i := 1; i < len(pt.Entries); i++ {
		if pt.Entries[i].GetMessageCount() > pt.Entries[i-1].GetMessageCount() {
			t.Errorf("entry %d count (%d) > entry %d count (%d)",
				i, pt.Entries[i].GetMessageCount(),
				i-1, pt.Entries[i-1].GetMessageCount())
		}
	}

	// Verify with both methods.
	msg, err := NewMessage(r.identity, pt)
	if err != nil {
		t.Fatal(err)
	}
	r.store.Add(msg)

	ok, err := r.store.VerifyParentTableByConstruction(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTableByConstruction returned false")
	}

	ok, err = r.store.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTable returned false")
	}
}

func BenchmarkWideDAG_VerifyParentTable(b *testing.B) {
	t := &testing.T{}
	r := buildWideDAG(t)
	pt := r.store.BuildParentTable()
	msg, err := NewMessage(r.identity, pt)
	if err != nil {
		b.Fatal(err)
	}
	r.store.Add(msg)

	for b.Loop() {
		ok, err := r.store.VerifyParentTable(msg)
		if err != nil || !ok {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkWideDAG_VerifyByConstruction(b *testing.B) {
	t := &testing.T{}
	r := buildWideDAG(t)
	pt := r.store.BuildParentTable()
	msg, err := NewMessage(r.identity, pt)
	if err != nil {
		b.Fatal(err)
	}
	r.store.Add(msg)

	for b.Loop() {
		ok, err := r.store.VerifyParentTableByConstruction(msg)
		if err != nil || !ok {
			b.Fatal("verification failed")
		}
	}
}
