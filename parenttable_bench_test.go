package braid

import "testing"

// TestVerifyParentTable_Correctness runs all the parent table test cases
// against VerifyParentTable to confirm it produces the same results as
// VerifyParentTableByConstruction.
func TestVerifyParentTable_Correctness(t *testing.T) {
	tests := []struct {
		name    string
		buildFn func(t *testing.T) (*Store, *Identity)
	}{
		{"LinearChain", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, gRef := addMsg(t, s, id, emptyParents())
			_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			return s, id
		}},
		{"Diamond", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, gRef := addMsg(t, s, id, emptyParents())
			_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			return s, id
		}},
		{"AsymmetricBranches", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, gRef := addMsg(t, s, id, emptyParents())
			_, cRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(cRef, 2)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			return s, id
		}},
		{"ThreeWayMerge", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, gRef := addMsg(t, s, id, emptyParents())
			_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			return s, id
		}},
		{"DeepDiamond", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, gRef := addMsg(t, s, id, emptyParents())
			_, xRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, yRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(xRef, 2)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(yRef, 2)))
			return s, id
		}},
		{"NoOverlap", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, g1Ref := addMsg(t, s, id, emptyParents())
			_, g2Ref := addMsg(t, s, id, emptyParents())
			_, _ = addMsg(t, s, id, parentTable(ptEntry(g1Ref, 1)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(g2Ref, 1)))
			return s, id
		}},
		{"ForwardVerification", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, fRef := addMsg(t, s, id, emptyParents())
			_, eRef := addMsg(t, s, id, parentTable(ptEntry(fRef, 1)))
			_, gRef := addMsg(t, s, id, parentTable(ptEntry(fRef, 1)))
			_, cRef := addMsg(t, s, id, parentTable(ptEntry(eRef, 2)))
			_, dRef := addMsg(t, s, id, parentTable(ptEntry(eRef, 2)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(cRef, 3), ptEntry(dRef, 1)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(cRef, 3), ptEntry(gRef, 1)))
			return s, id
		}},
		{"DeepSharedAncestor", func(t *testing.T) (*Store, *Identity) {
			s := NewStore()
			id := testID(t)
			_, gRef := addMsg(t, s, id, emptyParents())
			_, hRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, kRef := addMsg(t, s, id, parentTable(ptEntry(gRef, 1)))
			_, iRef := addMsg(t, s, id, parentTable(ptEntry(hRef, 2)))
			_, lRef := addMsg(t, s, id, parentTable(ptEntry(kRef, 2)))
			_, jRef := addMsg(t, s, id, parentTable(ptEntry(iRef, 3)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(jRef, 4)))
			_, _ = addMsg(t, s, id, parentTable(ptEntry(lRef, 3)))
			return s, id
		}},
		{"TestDAG", func(t *testing.T) (*Store, *Identity) {
			r := buildTestDAG(t)
			return r.store, r.identity
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, id := tt.buildFn(t)
			pt := s.BuildParentTable()
			msg, err := NewMessage(id, pt)
			if err != nil {
				t.Fatal(err)
			}
			s.Add(msg)

			okOld, err := s.VerifyParentTableByConstruction(msg)
			if err != nil {
				t.Fatal(err)
			}
			okNew, err := s.VerifyParentTable(msg)
			if err != nil {
				t.Fatal(err)
			}
			okConstruct, err := s.VerifyParentTableByConstruction(msg)
			if err != nil {
				t.Fatal(err)
			}

			if !okOld {
				t.Error("VerifyParentTableByConstruction returned false")
			}
			if !okNew {
				t.Error("VerifyParentTable returned false")
			}
			if !okConstruct {
				t.Error("VerifyParentTableByConstruction returned false")
			}
		})
	}
}

// TestVerifyParentTable_BadCount checks that the fast verifier rejects
// tampered counts.
func TestVerifyParentTable_BadCount(t *testing.T) {
	r := buildTestDAG(t)
	pt := r.store.BuildParentTable()

	badCount := pt.Entries[1].GetMessageCount() + 1
	pt.Entries[1].MessageCount = &badCount

	msg, err := NewMessage(r.identity, pt)
	if err != nil {
		t.Fatal(err)
	}
	r.store.Add(msg)

	ok, _ := r.store.VerifyParentTable(msg)
	if ok {
		t.Error("VerifyParentTable accepted a tampered count")
	}

	ok, _ = r.store.VerifyParentTableByConstruction(msg)
	if ok {
		t.Error("VerifyParentTableByConstruction accepted a tampered count")
	}
}

// benchSetup builds the test DAG and a message with its parent table,
// returning the store and message for benchmarking verification.
func benchSetup(b *testing.B) (*Store, *Message) {
	b.Helper()
	// Use a testing.T adapter for buildTestDAG.
	t := &testing.T{}
	r := buildTestDAG(t)
	pt := r.store.BuildParentTable()
	msg, err := NewMessage(r.identity, pt)
	if err != nil {
		b.Fatal(err)
	}
	r.store.Add(msg)
	return r.store, msg
}

func BenchmarkVerifyParentTable(b *testing.B) {
	s, msg := benchSetup(b)
	b.ResetTimer()
	for range b.N {
		ok, err := s.VerifyParentTable(msg)
		if err != nil || !ok {
			b.Fatal("verification failed")
		}
	}
}

func BenchmarkVerifyParentTableByConstruction(b *testing.B) {
	s, msg := benchSetup(b)
	b.ResetTimer()
	for range b.N {
		ok, err := s.VerifyParentTableByConstruction(msg)
		if err != nil || !ok {
			b.Fatal("verification failed")
		}
	}
}
