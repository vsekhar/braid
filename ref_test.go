package braid

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/vsekhar/braid/pkg/api/braidpb"
	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mustDecode(s string) []byte {
	h, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return h
}

func TestWriteTimestamp(t *testing.T) {
	const seconds = 42
	const nanos = 84
	const knownHash = "2eb8fa182cf0769228f406f7238de7b1babb479e50934fed597a967eec32bba5fbe992f76b0dded86d1d6985cb942f559ac1e6e1302a544aa567895d532be9a6"
	var knownHashBytes []byte = mustDecode(knownHash)

	goTime := time.Unix(seconds, nanos)
	pbTime := &timestamppb.Timestamp{
		Seconds: seconds,
		Nanos:   nanos,
	}
	h := sha3.NewShake256()
	writeTimestamp(h, goTime)
	hash1 := make([]byte, 64)
	h.Read(hash1)

	if !bytes.Equal(hash1, knownHashBytes) {
		t.Errorf("expected %s, got %s", hex.EncodeToString(knownHashBytes), hex.EncodeToString(hash1))
	}

	h.Reset()

	writeTimestamp(h, pbTime.AsTime())
	hash2 := make([]byte, 64)
	h.Read(hash2)

	if !bytes.Equal(hash1, hash2) {
		t.Error("hashes mismatch")
	}

	h.Reset()
	writeTimestamp(h, timestamppb.New(goTime).AsTime())
	h.Read(hash2)

	if !bytes.Equal(hash1, hash2) {
		t.Error("hashes mismatch")
	}
}

func TestRefPbToStruct(t *testing.T) {
	pb := &braidpb.Message{
		Authorship: &braidpb.Authorship{
			Signature: &braidpb.Signature{
				Ed25519V1: &braidpb.Ed25519KeyAndSignature{
					PublicKey: []byte{0, 1, 2, 3},
					Signature: []byte{4, 5, 6, 7},
				},
			},
			HorizonCommitment: &braidpb.HorizonCommitment{
				Ref: &braidpb.HorizonRef{
					Messages: &braidpb.MessageSetRef{
						Shake256_64V1: []byte{4, 2, 4},
					},
					Timestamp: timestamppb.Now(),
				},
				Application: &braidpb.ApplicationRef{Ref: []byte{9, 7, 29}},
			},
		},
		Timestamp: &braidpb.Timestamp{
			Timestamp: timestamppb.Now(),
		},
		Parentage: &braidpb.Parentage{
			Parents: []*braidpb.Parent{},
		},
		Data: &braidpb.ApplicationData{
			Data: []byte{100, 200, 255},
		},
	}

	m := FromProto(pb)
	h1 := Ref(m)
	h2 := RefPb(pb).Shake256_64V1
	if !bytes.Equal(h1, h2) {
		t.Error("pb-->struct ref hash mismatch")
	}
}

func TestRefStructToPb(t *testing.T) {
	m := &Message{
		author: Authorship{
			publicKey:   []byte{4, 56, 2},
			signature:   []byte{1, 4, 2},
			horizon:     time.Now(),
			ref:         []byte{5, 2, 2, 2},
			application: []byte{9, 2, 5},
		},
		timestamp:       time.Now(),
		applicationData: []byte{244, 214},
		self:            []byte{},
	}
	pb := toProto(m)
	h1 := Ref(m)
	h2 := RefPb(pb).Shake256_64V1
	if !bytes.Equal(h1, h2) {
		t.Error("struct-->pb ref hash mismatch")
	}
}
