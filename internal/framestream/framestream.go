package framestream

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	pb "github.com/vsekhar/braid/pkg/api/braidpb"
	"google.golang.org/protobuf/proto"
)

func readUvarint(r io.Reader) (uint64, error) {

	// https://developers.google.com/protocol-buffers/docs/encoding#varints

	buf := make([]byte, 0, 10)
	for {
		var b [1]byte
		if _, err := r.Read(b[:]); err != nil {
			return 0, err
		}
		buf = append(buf, b[0])
		if b[0]&(1<<7) == 0 || len(buf) >= binary.MaxVarintLen64 {
			break
		}
	}
	return binary.ReadUvarint(bytes.NewBuffer(buf))
}

func ReadFrame(r io.Reader) (*pb.Frame, error) {
	buf := &bytes.Buffer{}
	t := io.TeeReader(r, buf)

	// https://developers.google.com/protocol-buffers/docs/encoding#structure
	tag, err := readUvarint(t)
	if err != nil {
		return nil, err
	}

	// Embedded messages have wire type 2 "LEN"
	//
	// TODO: differentiate between string, bytes, embedded messages, and packed
	// repeated fields which all share wire type 2.
	if wireType := tag & 7; wireType != 2 {
		return nil, fmt.Errorf("bad wire tag: %d", tag)
	}

	// Read the length of the inner message
	len, err := readUvarint(t)
	if err != nil {
		return nil, err
	}
	frameBytes := make([]byte, buf.Len()+int(len))
	frameBytes = append(frameBytes[:0], buf.Bytes()...)
	if _, err := r.Read(frameBytes[buf.Len():]); err != nil {
		return nil, err
	}

	f := &pb.Frame{}
	if err := proto.Unmarshal(frameBytes, f); err != nil {
		return nil, err
	}
	return f, nil
}
