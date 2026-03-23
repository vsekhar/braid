package braid

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// WriteEnvelope writes a varint-length-prefixed Envelope to w.
func WriteEnvelope(w io.Writer, env *Envelope) error {
	data, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(len(data)))
	if _, err := w.Write(buf[:n]); err != nil {
		return fmt.Errorf("writing length prefix: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing envelope: %w", err)
	}
	return nil
}

// ReadEnvelope reads a varint-length-prefixed Envelope from r.
func ReadEnvelope(r io.Reader) (*Envelope, error) {
	br := bufio.NewReader(r)
	length, err := binary.ReadUvarint(br)
	if err != nil {
		return nil, fmt.Errorf("reading length prefix: %w", err)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(br, data); err != nil {
		return nil, fmt.Errorf("reading envelope body: %w", err)
	}
	env := &Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("unmarshaling envelope: %w", err)
	}
	return env, nil
}
