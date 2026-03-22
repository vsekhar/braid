package braid

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// HashMessage computes a MessageRef for msg using the sha256_v1 scheme.
// See braid.proto for the canonical encoding specification.
func HashMessage(msg *Message) (*MessageRef, error) {
	h := sha256.New()
	if err := hashMessageV1(h, msg); err != nil {
		return nil, err
	}
	return &MessageRef{Sha256V1: h.Sum(nil)}, nil
}

func hashMessageV1(h hash.Hash, msg *Message) error {
	// Timestamp.
	ts := msg.GetTimestamp()
	if ts == nil {
		return fmt.Errorf("message has no timestamp")
	}
	if err := binary.Write(h, binary.BigEndian, ts.GetSeconds()); err != nil {
		return err
	}
	if err := binary.Write(h, binary.BigEndian, ts.GetNanos()); err != nil {
		return err
	}

	// Author.
	authorBytes := msg.GetAuthor().GetEd25519V1()
	if authorBytes == nil {
		return fmt.Errorf("message has no author")
	}
	h.Write(authorBytes)

	// Parent table.
	for _, entry := range msg.GetParents().GetEntries() {
		parentHash := entry.GetParent().GetSha256V1()
		if parentHash == nil {
			return fmt.Errorf("parent entry has no sha256_v1 hash")
		}
		h.Write(parentHash)
		if err := binary.Write(h, binary.BigEndian, entry.GetMessageCount()); err != nil {
			return err
		}
	}

	return nil
}

// signingBytes returns the canonical byte representation of a message
// for signing and verification. Uses the same encoding as hashMessageV1.
func signingBytes(msg *Message) ([]byte, error) {
	h := sha256.New()
	if err := hashMessageV1(h, msg); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// NewMessage constructs a signed Message with the given parents.
func NewMessage(id *Identity, parents *ParentTable) (*Message, error) {
	msg := &Message{
		Timestamp: timestamppb.Now(),
		Parents:   parents,
		Author:    id.PublicKey(),
	}
	data, err := signingBytes(msg)
	if err != nil {
		return nil, fmt.Errorf("computing signing bytes: %w", err)
	}
	msg.Signature = id.Sign(data)
	return msg, nil
}

// VerifyMessageSignature checks that the message's signature is valid for its author.
func VerifyMessageSignature(msg *Message) (bool, error) {
	if msg.GetAuthor() == nil || msg.GetSignature() == nil {
		return false, fmt.Errorf("message missing author or signature")
	}
	data, err := signingBytes(msg)
	if err != nil {
		return false, fmt.Errorf("computing signing bytes: %w", err)
	}
	return VerifySignature(msg.Author, data, msg.Signature), nil
}
