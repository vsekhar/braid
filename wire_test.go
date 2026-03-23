package braid

import (
	"testing"

	"github.com/vsekhar/braid/internal/testproto"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// TestUnknownEnvelopeField verifies the protobuf library's behavior when
// oneof fields are extended, namely that an unknown oneof field value will
// be preserved, its binary data available via ProtoReflect.GetUnknown, and
// that a field number, wire time and length can be parsed parsed from the
// binary data with ConsumeField.
func TestUnknownEnvelopeField(t *testing.T) {
	// Marshal an ExtendedEnvelope with a field (number 4) unknown to Envelope.
	extended := &testproto.ExtendedEnvelope{
		Body: &testproto.ExtendedEnvelope_Future{
			Future: &testproto.FutureMessage{Payload: proto.String("hello from the future")},
		},
	}
	data, err := proto.Marshal(extended)
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal as the current Envelope (which has no field 4).
	env := &Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		t.Fatal(err)
	}

	// Body should be nil since field 4 is not recognized.
	if env.Body != nil {
		t.Fatalf("expected nil Body, got %T", env.Body)
	}

	// Unknown fields should contain the unrecognized field.
	unknown := env.ProtoReflect().GetUnknown()
	if len(unknown) == 0 {
		t.Fatal("expected unknown fields, got none")
	}

	num, typ, length := protowire.ConsumeField(unknown)
	if length < 0 {
		t.Fatalf("protowire.ConsumeField failed: length=%d", length)
	}
	if num != 100 {
		t.Errorf("expected field number 100, got %d", num)
	}

	t.Logf("unknown field: number=%d wire_type=%d length=%d", num, typ, length)
}
