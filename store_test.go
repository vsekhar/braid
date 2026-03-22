package braid

import (
	"testing"
)

func TestStore_CreateMessage(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// Create a message on an empty store (genesis).
	msg, ref, err := s.CreateMessage(id)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || ref == nil {
		t.Fatal("CreateMessage returned nil")
	}

	// Should be incorporated.
	if s.Len() != 1 {
		t.Errorf("store has %d messages, want 1", s.Len())
	}
	if s.PendingLen() != 0 {
		t.Errorf("store has %d pending, want 0", s.PendingLen())
	}

	// Genesis message: empty parent table, Total = 1.
	if Total(msg) != 1 {
		t.Errorf("Total(genesis) = %d, want 1", Total(msg))
	}

	// Should be on the frontier.
	frontier := s.Frontier()
	if len(frontier) != 1 {
		t.Fatalf("frontier has %d messages, want 1", len(frontier))
	}
	if refKey(frontier[0]) != refKey(ref) {
		t.Errorf("frontier message %s != created message %s",
			refKey(frontier[0])[:8], refKey(ref)[:8])
	}

	// Verify signature.
	ok, err := VerifyMessageSignature(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("signature verification failed")
	}

	// Verify parent table (trivially correct for genesis).
	ok, err = s.VerifyParentTable(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("parent table verification failed")
	}

	t.Logf("genesis: ref=%s total=%d", refKey(ref)[:8], Total(msg))
}

func TestStore_CreateMessageChain(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// Create a chain of 5 messages.
	var lastRef *MessageRef
	for i := range 5 {
		msg, ref, err := s.CreateMessage(id)
		if err != nil {
			t.Fatalf("message %d: %v", i, err)
		}

		// Each message should have exactly 1 parent (the previous frontier tip).
		entries := msg.GetParents().GetEntries()
		if i == 0 {
			if len(entries) != 0 {
				t.Errorf("message %d: got %d parents, want 0", i, len(entries))
			}
		} else {
			if len(entries) != 1 {
				t.Errorf("message %d: got %d parents, want 1", i, len(entries))
			}
			if len(entries) == 1 && refKey(entries[0].GetParent()) != refKey(lastRef) {
				t.Errorf("message %d: parent ref mismatch", i)
			}
			if len(entries) == 1 && entries[0].GetMessageCount() != uint64(i) {
				t.Errorf("message %d: parent count = %d, want %d",
					i, entries[0].GetMessageCount(), i)
			}
		}

		// Total should be i+1.
		if Total(msg) != uint64(i+1) {
			t.Errorf("message %d: Total = %d, want %d", i, Total(msg), i+1)
		}

		// Verify.
		ok, err := VerifyMessageSignature(msg)
		if err != nil || !ok {
			t.Errorf("message %d: signature verification failed", i)
		}
		ok, err = s.VerifyParentTable(msg)
		if err != nil || !ok {
			t.Errorf("message %d: parent table verification failed", i)
		}

		lastRef = ref
	}

	// Store should have 5 messages, 1 frontier.
	if s.Len() != 5 {
		t.Errorf("store has %d messages, want 5", s.Len())
	}
	frontier := s.Frontier()
	if len(frontier) != 1 {
		t.Errorf("frontier has %d messages, want 1", len(frontier))
	}
}

func TestStore_CreateMessageBranching(t *testing.T) {
	s := NewStore()
	id := testID(t)

	// Create genesis.
	_, genesisRef, err := s.CreateMessage(id)
	if err != nil {
		t.Fatal(err)
	}

	// Create two branches by manually adding messages with the same parent.
	branchA, err := NewMessage(id, parentTable(ptEntry(genesisRef, 1)))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.Add(branchA)
	if err != nil {
		t.Fatal(err)
	}

	branchB, err := NewMessage(id, parentTable(ptEntry(genesisRef, 1)))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.Add(branchB)
	if err != nil {
		t.Fatal(err)
	}

	// Frontier should have 2 messages.
	frontier := s.Frontier()
	if len(frontier) != 2 {
		t.Fatalf("frontier has %d messages, want 2", len(frontier))
	}

	// Create a merge message using CreateMessage — should have 2 parents.
	mergeMsg, mergeRef, err := s.CreateMessage(id)
	if err != nil {
		t.Fatal(err)
	}

	entries := mergeMsg.GetParents().GetEntries()
	if len(entries) != 2 {
		t.Fatalf("merge message has %d parents, want 2", len(entries))
	}

	// First parent: Total = 2 (genesis + branch).
	if entries[0].GetMessageCount() != 2 {
		t.Errorf("first parent count = %d, want 2", entries[0].GetMessageCount())
	}

	// Second parent: additional = 1 (just the other branch, genesis is shared).
	if entries[1].GetMessageCount() != 1 {
		t.Errorf("second parent count = %d, want 1", entries[1].GetMessageCount())
	}

	// Total of merge = 2 + 1 + 1 = 4 (genesis + branchA + branchB + merge).
	if Total(mergeMsg) != 4 {
		t.Errorf("Total(merge) = %d, want 4", Total(mergeMsg))
	}

	// Sum of parent counts + 1 = total messages in store.
	var sum uint64
	for _, e := range entries {
		sum += e.GetMessageCount()
	}
	if int(sum)+1 != s.Len() {
		t.Errorf("sum of counts + 1 = %d, store.Len = %d", sum+1, s.Len())
	}

	// Verify.
	ok, err := VerifyMessageSignature(mergeMsg)
	if err != nil || !ok {
		t.Error("merge message signature verification failed")
	}
	ok, err = s.VerifyParentTable(mergeMsg)
	if err != nil || !ok {
		t.Error("merge message parent table verification failed")
	}

	// Frontier should now be just the merge message.
	frontier = s.Frontier()
	if len(frontier) != 1 {
		t.Errorf("frontier has %d messages, want 1", len(frontier))
	}
	if refKey(frontier[0]) != refKey(mergeRef) {
		t.Errorf("frontier message != merge message")
	}

	t.Logf("merge: ref=%s total=%d parents=%d",
		refKey(mergeRef)[:8], Total(mergeMsg), len(entries))
}
