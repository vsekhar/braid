package braid

import (
	"fmt"
	"testing"
)

// testDAG builds a large, interesting DAG for testing. The structure:
//
//	                    (genesis)
//	                       |
//	                  [trunk: 100 messages]
//	                       |
//	              early branch point
//	             /                    \
//	   [left: 50 msgs]         [right: 50 msgs]
//	         |                        |
//	         +--- diamond1 ---+       |
//	         |                |       |
//	   [left: 20 msgs]  [mid: 20]    |
//	         |                |       |
//	         +--- diamond2 ---+       |
//	         |                        |
//	   [left: 30 msgs]               |
//	         |                  [right: 30 msgs]
//	              late branch point
//	             /                    \
//	   [late-left: 10 msgs]   [late-right: 10 msgs]
//
// Total messages: ~320
// Properties exercised:
//   - Long shared trunk (100 messages)
//   - Early branching (two long parallel branches)
//   - Diamond patterns (branches that diverge and reconverge)
//   - Late branching (near the frontier)
//   - Multiple frontier messages
type testDAGResult struct {
	store    *Store
	identity *Identity

	genesis  *MessageRef
	trunkTip *MessageRef // end of shared trunk

	// Early branches
	leftAfterDiamond2 *MessageRef // tip of left branch after second diamond
	rightTip          *MessageRef // tip of right branch

	// Late branches (the frontier)
	lateLeftTip  *MessageRef
	lateRightTip *MessageRef

	// Diamond merge points
	diamond1Merge *MessageRef
	diamond2Merge *MessageRef

	totalMessages int
}

func buildTestDAG(t *testing.T) *testDAGResult {
	t.Helper()
	s := NewStore()
	id := testID(t)
	r := &testDAGResult{store: s, identity: id}

	// Helper to build a chain of n messages from a given parent.
	// Returns the ref of the last message in the chain.
	buildChain := func(parentRef *MessageRef, parentTotal uint64, n int) *MessageRef {
		ref := parentRef
		total := parentTotal
		for range n {
			_, newRef := addMsg(t, s, id, parentTable(ptEntry(ref, total)))
			total = total + 1
			ref = newRef
		}
		return ref
	}

	// Genesis
	_, genesisRef := addMsg(t, s, id, emptyParents())
	r.genesis = genesisRef
	count := 1

	// Shared trunk: 100 messages
	trunkTip := buildChain(genesisRef, 1, 100)
	r.trunkTip = trunkTip
	count += 100
	trunkTotal := uint64(101) // genesis + 100

	// Early branch point: left and right diverge from trunk tip.

	// Left branch: 50 messages
	leftTip := buildChain(trunkTip, trunkTotal, 50)
	count += 50
	leftTotal := trunkTotal + 50 // 151

	// Right branch: 50 messages (parallel to left)
	rightTip := buildChain(trunkTip, trunkTotal, 50)
	count += 50
	rightTotal := trunkTotal + 50 // 151

	// Diamond 1: left branch splits into left-cont and mid, then reconverges.
	// left-cont: 20 messages from leftTip
	leftCont1 := buildChain(leftTip, leftTotal, 20)
	count += 20
	leftCont1Total := leftTotal + 20 // 171

	// mid: 20 messages from leftTip (parallel to left-cont)
	midTip := buildChain(leftTip, leftTotal, 20)
	count += 20
	_ = leftTotal + 20 // midTotal = 171 (not needed, mid's additional is used instead)

	// Diamond 1 merge: message with parents leftCont1 and midTip
	midAdditional := uint64(20) // mid's 20 unique messages
	_, diamond1Merge := addMsg(t, s, id, parentTable(
		ptEntry(leftCont1, leftCont1Total),
		ptEntry(midTip, midAdditional),
	))
	r.diamond1Merge = diamond1Merge
	count++
	diamond1Total := leftCont1Total + midAdditional + 1 // 192

	// Diamond 2: split again from diamond1Merge
	// left-cont2: 20 messages
	leftCont2 := buildChain(diamond1Merge, diamond1Total, 20)
	count += 20
	leftCont2Total := diamond1Total + 20 // 212

	// mid2: 20 messages
	mid2Tip := buildChain(diamond1Merge, diamond1Total, 20)
	count += 20
	_ = diamond1Total + 20 // mid2Total = 212 (not needed, mid2's additional is used instead)

	// Diamond 2 merge
	mid2Additional := uint64(20)
	_, diamond2Merge := addMsg(t, s, id, parentTable(
		ptEntry(leftCont2, leftCont2Total),
		ptEntry(mid2Tip, mid2Additional),
	))
	r.diamond2Merge = diamond2Merge
	count++
	diamond2Total := leftCont2Total + mid2Additional + 1 // 233

	// Left continues: 30 more messages after diamond 2
	leftFinal := buildChain(diamond2Merge, diamond2Total, 30)
	r.leftAfterDiamond2 = leftFinal
	count += 30
	leftFinalTotal := diamond2Total + 30 // 263

	// Right continues: 30 more messages
	rightFinal := buildChain(rightTip, rightTotal, 30)
	r.rightTip = rightFinal
	count += 30
	rightFinalTotal := rightTotal + 30 // 181

	// Late merge: left and right reconverge
	rightAdditional := rightFinalTotal - trunkTotal // 80 unique right messages
	_, lateMerge := addMsg(t, s, id, parentTable(
		ptEntry(leftFinal, leftFinalTotal),
		ptEntry(rightFinal, rightAdditional),
	))
	count++
	lateMergeTotal := leftFinalTotal + rightAdditional + 1 // 344

	// Late branch: split near the frontier
	lateLeft := buildChain(lateMerge, lateMergeTotal, 10)
	r.lateLeftTip = lateLeft
	count += 10

	lateRight := buildChain(lateMerge, lateMergeTotal, 10)
	r.lateRightTip = lateRight
	count += 10

	r.totalMessages = count
	return r
}

func TestTestDAG_Structure(t *testing.T) {
	r := buildTestDAG(t)

	// Verify total message count.
	if r.store.Len() != r.totalMessages {
		t.Errorf("store has %d messages, want %d", r.store.Len(), r.totalMessages)
	}

	// Frontier should be {lateLeftTip, lateRightTip}.
	frontier := r.store.Frontier()
	if len(frontier) != 2 {
		t.Fatalf("frontier has %d messages, want 2", len(frontier))
	}

	// Verify no pending messages.
	if r.store.PendingLen() != 0 {
		t.Errorf("store has %d pending messages, want 0", r.store.PendingLen())
	}

	t.Logf("DAG built: %d messages, %d frontier, %d pending",
		r.store.Len(), len(frontier), r.store.PendingLen())
}

func TestTestDAG_BuildParentTable(t *testing.T) {
	r := buildTestDAG(t)

	pt := r.store.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	// Log the parent table for inspection.
	for i, entry := range pt.Entries {
		t.Logf("entry %d: count=%d ref=%s",
			i, entry.GetMessageCount(), refKey(entry.GetParent())[:8])
	}

	// Sum of counts should equal total messages in the DAG.
	var sum uint64
	for _, entry := range pt.Entries {
		sum += entry.GetMessageCount()
	}
	if int(sum) != r.totalMessages {
		t.Errorf("sum of parent table counts = %d, want %d", sum, r.totalMessages)
	}

	// First entry count >= second entry count (ordering invariant).
	if pt.Entries[0].GetMessageCount() < pt.Entries[1].GetMessageCount() {
		t.Errorf("entries not ordered: %d < %d",
			pt.Entries[0].GetMessageCount(), pt.Entries[1].GetMessageCount())
	}

	// Verify the parent table.
	msg, err := NewMessage(r.identity, pt)
	if err != nil {
		t.Fatal(err)
	}
	r.store.Add(msg)
	ok, err := r.store.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTable returned false for test DAG")
	}
}

func TestTestDAG_Totals(t *testing.T) {
	r := buildTestDAG(t)

	// Spot-check some Total() values.
	genesis, ok := r.store.Get(r.genesis)
	if !ok {
		t.Fatal("genesis not found")
	}
	if Total(genesis) != 1 {
		t.Errorf("Total(genesis) = %d, want 1", Total(genesis))
	}

	trunkTip, ok := r.store.Get(r.trunkTip)
	if !ok {
		t.Fatal("trunkTip not found")
	}
	if Total(trunkTip) != 101 {
		t.Errorf("Total(trunkTip) = %d, want 101", Total(trunkTip))
	}

	lateLeft, ok := r.store.Get(r.lateLeftTip)
	if !ok {
		t.Fatal("lateLeftTip not found")
	}
	lateRight, ok := r.store.Get(r.lateRightTip)
	if !ok {
		t.Fatal("lateRightTip not found")
	}
	// Both late tips have the same total (symmetric late branch).
	if Total(lateLeft) != Total(lateRight) {
		t.Errorf("Total(lateLeft)=%d != Total(lateRight)=%d",
			Total(lateLeft), Total(lateRight))
	}

	t.Logf("Total(genesis)=%d Total(trunkTip)=%d Total(lateLeftTip)=%d",
		Total(genesis), Total(trunkTip), Total(lateLeft))
}

func TestTestDAG_Children(t *testing.T) {
	r := buildTestDAG(t)

	// Trunk tip should have 2 children (start of left and right branches).
	kids := r.store.Children(r.trunkTip)
	if len(kids) != 2 {
		t.Errorf("trunkTip has %d children, want 2", len(kids))
	}

	// Genesis should have 1 child (first trunk message).
	kids = r.store.Children(r.genesis)
	if len(kids) != 1 {
		t.Errorf("genesis has %d children, want 1", len(kids))
	}

	// Diamond merge points should have children.
	kids = r.store.Children(r.diamond1Merge)
	if len(kids) < 2 {
		t.Errorf("diamond1Merge has %d children, want >= 2", len(kids))
	}
}

func TestTestDAG_MessageCounts(t *testing.T) {
	r := buildTestDAG(t)

	// Verify specific message counts by building a parent table and
	// checking counts match expectations.
	pt := r.store.BuildParentTable()

	// The two frontier messages are lateLeftTip and lateRightTip.
	// Both descend from the late merge point.
	// lateLeftTip: Total = lateMergeTotal + 10
	// lateRightTip: Total = lateMergeTotal + 10
	// First parent count = Total(winner)
	// Second parent additional = 10 (just its 10 unique messages)

	first := pt.Entries[0].GetMessageCount()
	second := pt.Entries[1].GetMessageCount()

	expectedFirst := uint64(r.totalMessages) - 10 // one tip's total
	if first != expectedFirst {
		t.Errorf("first parent count = %d, want %d", first, expectedFirst)
	}
	if second != 10 {
		t.Errorf("second parent count = %d, want 10", second)
	}

	t.Logf("Parent table: [%d, %d] (total messages: %d)",
		first, second, r.totalMessages)

	// Cross-check: sum should equal total messages.
	if first+second != uint64(r.totalMessages) {
		fmt.Printf("first=%d + second=%d = %d, totalMessages=%d\n",
			first, second, first+second, r.totalMessages)
		t.Errorf("sum %d != totalMessages %d", first+second, r.totalMessages)
	}
}

func TestTestDAG_VerifyEachEntry(t *testing.T) {
	r := buildTestDAG(t)

	pt := r.store.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	// --- Entry 0: the frontier message with the most ancestors ---
	//
	// Both lateLeftTip and lateRightTip have the same Total (they are
	// symmetric: 10 messages each after the late merge). The first parent
	// is whichever wins the tiebreak.
	//
	// Total of either tip:
	//   genesis(1) + trunk(100) + left(50) + right(50)
	//   + diamond1-left(20) + diamond1-mid(20) + diamond1-merge(1)
	//   + diamond2-left(20) + diamond2-mid(20) + diamond2-merge(1)
	//   + left-final(30) + right-final(30) + late-merge(1) + own-branch(10)
	//   = 354
	//
	// Entry 0 message_count = Total(selected tip) = 354.

	entry0 := pt.Entries[0]
	if entry0.GetMessageCount() != 354 {
		t.Errorf("entry 0: count = %d, want 354", entry0.GetMessageCount())
	}

	// Verify entry 0's ref is one of the two frontier tips.
	entry0Key := refKey(entry0.GetParent())
	lateLeftKey := refKey(r.lateLeftTip)
	lateRightKey := refKey(r.lateRightTip)
	if entry0Key != lateLeftKey && entry0Key != lateRightKey {
		t.Errorf("entry 0 ref %s is neither lateLeftTip nor lateRightTip", entry0Key[:8])
	}
	t.Logf("entry 0: ref=%s count=%d (Total of selected tip)",
		entry0Key[:8], entry0.GetMessageCount())

	// Verify Total() of the referenced parent matches the count.
	entry0Msg, ok := r.store.Get(entry0.GetParent())
	if !ok {
		t.Fatal("entry 0 parent not found in store")
	}
	if Total(entry0Msg) != entry0.GetMessageCount() {
		t.Errorf("entry 0: Total(parent) = %d, count = %d",
			Total(entry0Msg), entry0.GetMessageCount())
	}

	// --- Entry 1: the other frontier tip ---
	//
	// Both tips share the late merge point and everything below it (344
	// messages). The second tip contributes only its 10 unique messages.
	//
	// Entry 1 message_count = additional(other tip | first tip) = 10.

	entry1 := pt.Entries[1]
	if entry1.GetMessageCount() != 10 {
		t.Errorf("entry 1: count = %d, want 10", entry1.GetMessageCount())
	}

	// Verify entry 1's ref is the OTHER frontier tip.
	entry1Key := refKey(entry1.GetParent())
	if entry0Key == lateLeftKey {
		if entry1Key != lateRightKey {
			t.Errorf("entry 1 ref %s should be lateRightTip", entry1Key[:8])
		}
	} else {
		if entry1Key != lateLeftKey {
			t.Errorf("entry 1 ref %s should be lateLeftTip", entry1Key[:8])
		}
	}
	t.Logf("entry 1: ref=%s count=%d (additional unique messages)",
		entry1Key[:8], entry1.GetMessageCount())

	// --- Verify the table as a whole ---

	// Ordering: entry 0 count >= entry 1 count.
	if entry0.GetMessageCount() < entry1.GetMessageCount() {
		t.Errorf("ordering violation: %d < %d",
			entry0.GetMessageCount(), entry1.GetMessageCount())
	}

	// Sum should equal total messages.
	sum := entry0.GetMessageCount() + entry1.GetMessageCount()
	if int(sum) != r.totalMessages {
		t.Errorf("sum of counts %d != totalMessages %d", sum, r.totalMessages)
	}

	// Build a message with this parent table and verify.
	msg, err := NewMessage(r.identity, pt)
	if err != nil {
		t.Fatal(err)
	}
	r.store.Add(msg)
	ok, err = r.store.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTable rejected a correctly built table")
	}
	t.Logf("VerifyParentTable: OK")
}
