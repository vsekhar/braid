package braid

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/vsekhar/braid/pkg/ed25519"
)

var privKey ed25519.PrivateKey

func TestMain(m *testing.M) {
	privKey = ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	os.Exit(m.Run())
}

func makeMessage(privateKey ed25519.PrivateKey) *Message {
	m := &Message{
		author: Authorship{
			publicKey: privateKey.Public().(ed25519.PublicKey),
		},
		timestamp:       time.Now(),
		applicationData: []byte{1, 1, 2, 3, 5, 8},
	}
	m.self = Ref(m)
	m.author.signature = ed25519.Sign(privateKey, m.self)
	return m
}

func TestProto(t *testing.T) {
	m := makeMessage(privKey)

	pbM := toProto(m)
	o2 := fromProto(pbM)

	if !m.Equal(o2.m) {
		t.Errorf("expected %+v", m)
		t.Errorf("got %+v", o2.m)
	}

	if !bytes.Equal(Ref(o2.m), o2.m.self) {
		t.Errorf("bad self hash")
	}
	if !bytes.Equal(RefPb(pbM).Shake256_64V1, o2.m.self) {
		t.Errorf("bad proto hash")
	}

	if !ed25519.Verify(o2.m.author.publicKey, o2.m.self, o2.m.author.signature) {
		t.Errorf("signature failed to verify")
	}
}
