package braid_test

import (
	"crypto/ed25519"
	"testing"

	"github.com/vsekhar/braid"
)

func TestBraid(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	b := &braid.Braid{}
	if b.Len() != 0 {
		t.Fatalf("expected length 0, got %d", b.Len())
	}
	b.WriteAndSign(priv, []byte{0, 1, 2, 3})
	if b.Len() != 1 {
		t.Errorf("expected length 1, got %d", b.Len())
	}
}
