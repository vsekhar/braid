package braid

import (
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// testID creates a deterministic test identity.
func testID(t *testing.T) *Identity {
	t.Helper()
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// addMsg creates a message with the given parent table, adds it to the store,
// and returns the message and its ref.
func addMsg(t *testing.T, s *Store, id *Identity, parents *ParentTable) (*Message, *MessageRef) {
	t.Helper()
	msg, err := NewMessage(id, parents)
	if err != nil {
		t.Fatal(err)
	}
	ref, _, err := s.Add(msg)
	if err != nil {
		t.Fatal(err)
	}
	return msg, ref
}

func emptyParents() *ParentTable {
	return &ParentTable{}
}

func parentTable(entries ...*ParentTable_Entry) *ParentTable {
	return &ParentTable{Entries: entries}
}

func ptEntry(ref *MessageRef, count uint64) *ParentTable_Entry {
	return &ParentTable_Entry{Parent: ref, MessageCount: &count}
}

func TestTotal(t *testing.T) {
	// Message with no parents: total = 1 (just itself).
	msg := &Message{Timestamp: timestamppb.Now(), Parents: emptyParents()}
	if got := Total(msg); got != 1 {
		t.Errorf("Total(no parents) = %d, want 1", got)
	}

	// Message with one parent (count=5): total = 5 + 1 = 6.
	c := uint64(5)
	msg = &Message{
		Timestamp: timestamppb.Now(),
		Parents: &ParentTable{Entries: []*ParentTable_Entry{
			{Parent: &MessageRef{Sha256V1: []byte("dummy")}, MessageCount: &c},
		}},
	}
	if got := Total(msg); got != 6 {
		t.Errorf("Total(one parent count=5) = %d, want 6", got)
	}

	// Message with two parents (count=10, count=3): total = 10 + 3 + 1 = 14.
	c1 := uint64(10)
	c2 := uint64(3)
	msg = &Message{
		Timestamp: timestamppb.Now(),
		Parents: &ParentTable{Entries: []*ParentTable_Entry{
			{Parent: &MessageRef{Sha256V1: []byte("p1")}, MessageCount: &c1},
			{Parent: &MessageRef{Sha256V1: []byte("p2")}, MessageCount: &c2},
		}},
	}
	if got := Total(msg); got != 14 {
		t.Errorf("Total(two parents count=10,3) = %d, want 14", got)
	}
}

// TestBuildParentTable_LinearChain tests a simple linear chain: G → A.
// Frontier = {A}. Parent table of a new message should have one entry
// with message_count = Total(A).
func TestBuildParentTable_LinearChain(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// G is genesis (no parents).
	_, gRef := addMsg(t, s, id, emptyParents())

	// A has parent G.
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	// Frontier should be {A}.
	pt := s.BuildParentTable()
	if len(pt.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(pt.Entries))
	}
	// A's total = 1 (G) + 1 (A) = 2. New message's first parent count = Total(A) = 2.
	if pt.Entries[0].GetMessageCount() != 2 {
		t.Errorf("first parent count = %d, want 2", pt.Entries[0].GetMessageCount())
	}
}

// TestBuildParentTable_Diamond tests a diamond DAG:
//
//	G
//	├── A
//	└── B
//
// Frontier = {A, B}. Both share parent G.
func TestBuildParentTable_Diamond(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// G is genesis.
	_, gRef := addMsg(t, s, id, emptyParents())

	// A and B both have parent G.
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	// Frontier = {A, B}. Both have Total = 2.
	pt := s.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	// First parent: Total = 2 (one of A or B).
	if pt.Entries[0].GetMessageCount() != 2 {
		t.Errorf("first parent count = %d, want 2", pt.Entries[0].GetMessageCount())
	}

	// Second parent: additional = 1 (just itself, since G is shared).
	if pt.Entries[1].GetMessageCount() != 1 {
		t.Errorf("second parent count = %d, want 1", pt.Entries[1].GetMessageCount())
	}
}

// TestBuildParentTable_AsymmetricBranches tests:
//
//	G → C → A
//	G → B
//
// A has Total=3, B has Total=2. First parent = A (count=3).
// Additional from B = 1 (just B, since G is shared).
func TestBuildParentTable_AsymmetricBranches(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, gRef := addMsg(t, s, id, emptyParents())
	_, cRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	// A has parent C (Total(A) = 1 + 1 + 1 = 3).
	_, _ = addMsg(t, s, id, parentTable(ptEntry(cRef, 2)))

	// B has parent G (Total(B) = 1 + 1 = 2).
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	pt := s.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	// First parent should be A (count=3, higher total).
	if pt.Entries[0].GetMessageCount() != 3 {
		t.Errorf("first parent count = %d, want 3", pt.Entries[0].GetMessageCount())
	}

	// Second parent B: additional = 1 (just B itself; G and C are covered by A).
	if pt.Entries[1].GetMessageCount() != 1 {
		t.Errorf("second parent count = %d, want 1", pt.Entries[1].GetMessageCount())
	}
}

// TestBuildParentTable_ThreeWayMerge tests:
//
//	G → A
//	G → B
//	G → C
//
// All have Total=2. First parent = any (count=2).
// Second parent = another (additional=1).
// Third parent = last (additional=1).
func TestBuildParentTable_ThreeWayMerge(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, gRef := addMsg(t, s, id, emptyParents())
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	pt := s.BuildParentTable()
	if len(pt.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(pt.Entries))
	}

	if pt.Entries[0].GetMessageCount() != 2 {
		t.Errorf("first parent count = %d, want 2", pt.Entries[0].GetMessageCount())
	}
	if pt.Entries[1].GetMessageCount() != 1 {
		t.Errorf("second parent count = %d, want 1", pt.Entries[1].GetMessageCount())
	}
	if pt.Entries[2].GetMessageCount() != 1 {
		t.Errorf("third parent count = %d, want 1", pt.Entries[2].GetMessageCount())
	}
}

// TestBuildParentTable_DeepDiamond tests:
//
//	G → X → A
//	G → Y → B
//
// A has Total=3, B has Total=3. Additional of B given A:
// reachable(A) = {A, X, G}, reachable(B) = {B, Y, G}.
// additional(B|A) = |{B, Y}| = 2.
func TestBuildParentTable_DeepDiamond(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, gRef := addMsg(t, s, id, emptyParents())
	_, xRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, yRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	// A has parent X.
	_, _ = addMsg(t, s, id, parentTable(ptEntry(xRef, 2)))
	// B has parent Y.
	_, _ = addMsg(t, s, id, parentTable(ptEntry(yRef, 2)))

	pt := s.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	if pt.Entries[0].GetMessageCount() != 3 {
		t.Errorf("first parent count = %d, want 3", pt.Entries[0].GetMessageCount())
	}
	if pt.Entries[1].GetMessageCount() != 2 {
		t.Errorf("second parent count = %d, want 2", pt.Entries[1].GetMessageCount())
	}
}

// TestBuildParentTable_NoOverlap tests completely disjoint branches
// (two separate genesis messages).
func TestBuildParentTable_NoOverlap(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// Two independent genesis messages → two independent chains.
	_, g1Ref := addMsg(t, s, id, emptyParents())
	_, g2Ref := addMsg(t, s, id, emptyParents())
	_, _ = addMsg(t, s, id, parentTable(ptEntry(g1Ref, 1)))
	_, _ = addMsg(t, s, id, parentTable(ptEntry(g2Ref, 1)))

	pt := s.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	// No overlap: first=2, second=2.
	if pt.Entries[0].GetMessageCount() != 2 {
		t.Errorf("first parent count = %d, want 2", pt.Entries[0].GetMessageCount())
	}
	if pt.Entries[1].GetMessageCount() != 2 {
		t.Errorf("second parent count = %d, want 2", pt.Entries[1].GetMessageCount())
	}
}

// TestBuildParentTable_SingleMessage tests a frontier with one message
// that is genesis (no parents).
func TestBuildParentTable_SingleMessage(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, _ = addMsg(t, s, id, emptyParents())

	pt := s.BuildParentTable()
	if len(pt.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(pt.Entries))
	}
	if pt.Entries[0].GetMessageCount() != 1 {
		t.Errorf("first parent count = %d, want 1", pt.Entries[0].GetMessageCount())
	}
}

// TestBuildParentTable_Empty tests an empty store.
func TestBuildParentTable_Empty(t *testing.T) {
	s := NewStore()
	pt := s.BuildParentTable()
	if len(pt.Entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(pt.Entries))
	}
}

// TestVerifyParentTable tests that BuildParentTable produces tables that
// pass verification.
func TestVerifyParentTable(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, gRef := addMsg(t, s, id, emptyParents())
	_, xRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, yRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, _ = addMsg(t, s, id, parentTable(ptEntry(xRef, 2)))
	_, _ = addMsg(t, s, id, parentTable(ptEntry(yRef, 2)))

	// Build a parent table from the frontier.
	pt := s.BuildParentTable()

	// Create a message with this parent table and verify it.
	msg, err := NewMessage(id, pt)
	if err != nil {
		t.Fatal(err)
	}

	// Add the message so the store has it for verification context.
	_, _, err = s.Add(msg)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := s.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTable returned false for a correctly built table")
	}
}

// TestVerifyParentTable_BadCount tests that verification rejects a parent
// table with an incorrect count.
func TestVerifyParentTable_BadCount(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, gRef := addMsg(t, s, id, emptyParents())
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	// Build correct parent table, then tamper with the count.
	pt := s.BuildParentTable()
	badCount := pt.Entries[0].GetMessageCount() + 1
	pt.Entries[0].MessageCount = &badCount

	msg, err := NewMessage(id, pt)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(msg)

	ok, _ := s.VerifyParentTable(msg)
	if ok {
		t.Error("VerifyParentTable returned true for a tampered count")
	}
}

// TestBuildParentTable_ForwardVerification tests the scenario where B's walk
// reaches a shared message before A's walk does, requiring forward
// verification to correct the count.
//
// DAG:
//
//	F (genesis)
//	├── E → C → P1 (parents: C, D)
//	│   └── D ──┘
//	└── G → X  (parents: C, G)
//	        └── C (shared)
//
// reachable(P1) = {P1, C, D, E, F} = 5
// reachable(X)  = {X, G, C, E, F} = 5
// additional(X | P1) = |{X, G}| = 2
//
// The tricky part: B's walk from X reaches F (through G→F) before A's walk
// from P1 reaches F (through C→E→F). F is a false positive that forward
// verification must catch.
func TestBuildParentTable_ForwardVerification(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// F: genesis
	_, fRef := addMsg(t, s, id, emptyParents())
	// E: parent F
	_, eRef := addMsg(t, s, id, parentTable(ptEntry(fRef, 1)))
	// G: parent F
	_, gRef := addMsg(t, s, id, parentTable(ptEntry(fRef, 1)))
	// C: parent E
	_, cRef := addMsg(t, s, id, parentTable(ptEntry(eRef, 2)))
	// D: parent E
	_, dRef := addMsg(t, s, id, parentTable(ptEntry(eRef, 2)))
	// P1: parents C(3), D(1)
	_, _ = addMsg(t, s, id, parentTable(ptEntry(cRef, 3), ptEntry(dRef, 1)))
	// X: parents C(3), G(1)
	_, _ = addMsg(t, s, id, parentTable(ptEntry(cRef, 3), ptEntry(gRef, 1)))

	// Frontier should be {P1, X}.
	frontier := s.Frontier()
	if len(frontier) != 2 {
		t.Fatalf("frontier has %d messages, want 2", len(frontier))
	}

	pt := s.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	// Both P1 and X have Total=5. First parent = either (count=5).
	if pt.Entries[0].GetMessageCount() != 5 {
		t.Errorf("first parent count = %d, want 5", pt.Entries[0].GetMessageCount())
	}

	// Second parent: additional = 2 (just the unique messages: the other
	// frontier message itself + its unique intermediate node).
	// If P1 selected first: additional(X|P1) = |{X, G}| = 2.
	// If X selected first: additional(P1|X) = |{P1, D}| = 2.
	if pt.Entries[1].GetMessageCount() != 2 {
		t.Errorf("second parent count = %d, want 2", pt.Entries[1].GetMessageCount())
	}

	// Verify the built table is valid.
	msg, err := NewMessage(id, pt)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(msg)
	ok, err := s.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTable returned false for forward-verification scenario")
	}
}

// TestBuildParentTable_DeepSharedAncestor tests a deeper version of the
// forward verification scenario where the shared ancestor is multiple
// hops away from both branches.
//
// DAG:
//
//	G (genesis)
//	├── H → I → J → A
//	└── K → L → B
//
// reachable(A) = {A, J, I, H, G} = 5
// reachable(B) = {B, L, K, G} = 4
// additional(B | A) = |{B, L, K}| = 3
func TestBuildParentTable_DeepSharedAncestor(t *testing.T) {
	s := NewStore()
	id := testID(t)

	_, gRef := addMsg(t, s, id, emptyParents())
	_, hRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, kRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
	_, iRef := addMsg(t, s, id, parentTable(ptEntry(hRef, 2)))
	_, lRef := addMsg(t, s, id, parentTable(ptEntry(kRef, 2)))
	_, jRef := addMsg(t, s, id, parentTable(ptEntry(iRef, 3)))
	// A: parent J
	_, _ = addMsg(t, s, id, parentTable(ptEntry(jRef, 4)))
	// B: parent L
	_, _ = addMsg(t, s, id, parentTable(ptEntry(lRef, 3)))

	pt := s.BuildParentTable()
	if len(pt.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(pt.Entries))
	}

	// First parent = A (Total=5, higher than B's Total=4).
	if pt.Entries[0].GetMessageCount() != 5 {
		t.Errorf("first parent count = %d, want 5", pt.Entries[0].GetMessageCount())
	}

	// Second parent = B, additional = 3 ({B, L, K}).
	if pt.Entries[1].GetMessageCount() != 3 {
		t.Errorf("second parent count = %d, want 3", pt.Entries[1].GetMessageCount())
	}

	msg, err := NewMessage(id, pt)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(msg)
	ok, err := s.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTable returned false for deep shared ancestor scenario")
	}
}

// TestBuildAndVerify_Ordering tests that entries are ordered by count
// descending and that verification accepts the ordering.
func TestBuildAndVerify_Ordering(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// Build a chain of length 5 and a short branch of length 2, sharing genesis.
	_, gRef := addMsg(t, s, id, emptyParents())
	ref := gRef
	for i := 0; i < 4; i++ {
		count := uint64(i + 1)
		_, r := addMsg(t, s, id, parentTable(ptEntry(ref, count)))
		ref = r
	}
	// Short branch from genesis.
	_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))

	pt := s.BuildParentTable()
	if len(pt.Entries) < 2 {
		t.Fatalf("got %d entries, want >= 2", len(pt.Entries))
	}

	// Verify ordering: each count <= previous.
	for i := 1; i < len(pt.Entries); i++ {
		if pt.Entries[i].GetMessageCount() > pt.Entries[i-1].GetMessageCount() {
			t.Errorf("entry %d count (%d) > entry %d count (%d)",
				i, pt.Entries[i].GetMessageCount(),
				i-1, pt.Entries[i-1].GetMessageCount())
		}
	}

	// Build message and verify.
	msg, err := NewMessage(id, pt)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(msg)
	ok, err := s.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("VerifyParentTable returned false")
	}
}
