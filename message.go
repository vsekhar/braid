package braid

import (
	stdbytes "bytes"
	"time"

	pb "github.com/vsekhar/braid/pkg/api/braidpb"
	"github.com/vsekhar/braid/pkg/ed25519"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Authorship struct {
	publicKey   ed25519.PublicKey
	signature   []byte
	horizon     time.Time
	ref         []byte // hash([]horizon_msgs)
	application []byte // hash(application_data)
}

type Parent struct {
	msg        *Message
	relContrib uint64
}

// Message is a message in the Braid.
type Message struct {
	author          Authorship
	timestamp       time.Time
	parents         []Parent
	applicationData []byte
	self            string // just the raw bytes as a string, no encoding
}

// toProto converts a Message to a pb.Message.
//
// The argument m must not be modified after calling toProto.
func toProto(m *Message) *pb.Message {
	pbMsg := &pb.Message{
		Data: &pb.ApplicationData{
			Data: m.applicationData,
		},
		Parentage: &pb.Parentage{
			Parents: make([]*pb.Parent, 0, len(m.parents)),
		},
		Timestamp: &pb.Timestamp{
			Timestamp: timestamppb.New(m.timestamp),
		},
		Authorship: &pb.Authorship{
			Signature: &pb.Signature{
				Ed25519V1: &pb.Ed25519KeyAndSignature{
					PublicKey: m.author.publicKey,
					Signature: m.author.signature,
				},
			},
			HorizonCommitment: &pb.HorizonCommitment{
				Ref: &pb.HorizonRef{
					Messages: &pb.MessageSetRef{
						Shake256_64V1: m.author.ref,
					},
					Timestamp: timestamppb.New(m.author.horizon),
				},
				Application: &pb.ApplicationRef{Ref: m.author.application},
			},
		},
	}
	for _, p := range m.parents {
		pbMsg.Parentage.Parents = append(pbMsg.Parentage.Parents, &pb.Parent{
			Ref: &pb.MessageRef{
				Shake256_64V1: []byte(p.msg.self),
			},
		})
	}
	return pbMsg
}

// FromProto converts a pb.Message to an orphan.
//
// The argument pbMsg must not be modified after calling FromProto.
func fromProto(pbMsg *pb.Message) *orphan {
	m := &Message{
		applicationData: pbMsg.Data.Data,
		timestamp:       pbMsg.Timestamp.Timestamp.AsTime(),
		parents:         make([]Parent, len(pbMsg.Parentage.Parents)),
		author: Authorship{
			publicKey:   pbMsg.Authorship.Signature.Ed25519V1.PublicKey,
			signature:   pbMsg.Authorship.Signature.Ed25519V1.Signature,
			horizon:     pbMsg.Authorship.HorizonCommitment.Ref.Timestamp.AsTime(),
			ref:         pbMsg.Authorship.HorizonCommitment.Ref.Messages.Shake256_64V1,
			application: pbMsg.Authorship.HorizonCommitment.Application.Ref,
		},
	}
	m.self = string(Ref(m))
	o := &orphan{
		Message:    m,
		parentRefs: make([][]byte, len(m.parents)),
	}
	for i, parent := range pbMsg.Parentage.Parents {
		m.parents[i].relContrib = parent.Contribution
		o.parentRefs[i] = parent.Ref.Shake256_64V1
	}
	return o
}

func (m1 *Message) Equal(m2 *Message) bool {
	// Cannot use deep equals because parent.msg and self may be nil.

	if !stdbytes.Equal(m1.author.publicKey, m2.author.publicKey) {
		return false
	}
	if !stdbytes.Equal(m1.author.signature, m2.author.signature) {
		return false
	}
	if !stdbytes.Equal(m1.author.ref, m2.author.ref) {
		return false
	}
	if !stdbytes.Equal(m1.author.application, m2.author.application) {
		return false
	}
	if !m1.timestamp.Equal(m2.timestamp) {
		return false
	}
	if len(m1.parents) != len(m2.parents) {
		return false
	}
	for i, p1 := range m1.parents {
		p2 := m2.parents[i]
		if p1.msg.self != p2.msg.self {
			return false
		}
		if p1.relContrib != p2.relContrib {
			return false
		}
	}
	return stdbytes.Equal(m1.applicationData, m2.applicationData)
	// Intentionally skip checking self field.
}
