package braid

import (
	"encoding/binary"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// WriteEnvelope writes a length-prefixed Envelope to w.
func WriteEnvelope(w io.Writer, env *Envelope) error {
	data, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(data))); err != nil {
		return fmt.Errorf("writing length prefix: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing envelope: %w", err)
	}
	return nil
}

// ReadEnvelope reads a length-prefixed Envelope from r.
func ReadEnvelope(r io.Reader) (*Envelope, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("reading length prefix: %w", err)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("reading envelope body: %w", err)
	}
	env := &Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("unmarshaling envelope: %w", err)
	}
	return env, nil
}
