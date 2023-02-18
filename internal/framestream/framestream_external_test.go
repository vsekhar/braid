package framestream_test

import (
	"bytes"
	"testing"

	"github.com/vsekhar/braid/internal/framestream"
	"github.com/vsekhar/braid/pkg/api/braidpb"
	"google.golang.org/protobuf/proto"
)

func TestFrameStream(t *testing.T) {
	f := &braidpb.Frame{
		Payload: &braidpb.Frame_LastField{},
	}

	b, err := proto.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	buf := bytes.NewBuffer(b)
	f2, err := framestream.ReadFrameFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	switch f2.Payload.(type) {
	case *braidpb.Frame_LastField:
	default:
		t.Error("bad type")
	}
}
