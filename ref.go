package braid

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/vsekhar/braid/internal/oneof"
	pb "github.com/vsekhar/braid/pkg/api/braidpb"
	"golang.org/x/crypto/sha3"
)

func writeTimestamp(w io.Writer, t time.Time) {
	// Timestamp contains seconds (int64) and nanos (int32)
	var bs [8]byte
	seconds := t.Unix()
	nanos := t.Nanosecond()
	if nanos > 1e9 {
		panic("bad timestamp")
	}
	binary.LittleEndian.PutUint64(bs[:], uint64(seconds))
	if _, err := w.Write(bs[:]); err != nil {
		panic(err)
	}
	binary.LittleEndian.PutUint32(bs[:4], uint32(nanos))
	if _, err := w.Write(bs[:4]); err != nil {
		panic(err)
	}
}

func Ref(m *Message) []byte {
	h := sha3.NewShake256()
	h.Write(m.author.publicKey)
	// Omit signature
	if !m.author.horizon.Equal(time.Time{}) {
		writeTimestamp(h, m.author.horizon)
	}
	h.Write(m.author.ref)
	h.Write(m.author.application)
	if !m.timestamp.Equal(time.Time{}) {
		writeTimestamp(h, m.timestamp)
	}
	for _, a := range m.parents {
		switch a.Which() {
		case oneof.First:
			h.Write(a.Get1())
		case oneof.Second:
			h.Write(a.Get2().self)
		default:
			panic("bad parent ref")
		}
	}
	h.Write(m.applicationData)
	hash := make([]byte, 64)
	h.Read(hash)
	return hash
}

func RefPb(e *pb.Message) *pb.MessageRef {
	h := sha3.NewShake256()
	if e.Authorship != nil {
		if e.Authorship.Signature != nil {
			if e.Authorship.Signature.Ed25519V1 != nil {
				h.Write(e.Authorship.Signature.Ed25519V1.PublicKey)
				// Omit signature
			}
		}
		if e.Authorship.HorizonCommitment != nil {
			if e.Authorship.HorizonCommitment.Ref != nil {
				if e.Authorship.HorizonCommitment.Ref.Timestamp != nil && !e.Authorship.HorizonCommitment.Ref.Timestamp.AsTime().Equal(time.Time{}) {
					writeTimestamp(h, e.Authorship.HorizonCommitment.Ref.Timestamp.AsTime())
				}
				h.Write(e.Authorship.HorizonCommitment.Ref.Messages.Shake256_64V1)
			}
			if e.Authorship.HorizonCommitment.Application != nil {
				h.Write(e.Authorship.HorizonCommitment.Application.Ref)
			}
		}
	}
	if e.Timestamp != nil && !e.Timestamp.Timestamp.AsTime().Equal(time.Time{}) {
		writeTimestamp(h, e.Timestamp.Timestamp.AsTime())
	}
	if e.Parentage != nil {
		for _, a := range e.Parentage.Parents {
			h.Write(a.Ref.Shake256_64V1)
		}
	}
	if e.Data != nil {
		h.Write(e.Data.Data)
	}
	hash := make([]byte, 64)
	h.Read(hash)
	return &pb.MessageRef{Shake256_64V1: hash}
}
