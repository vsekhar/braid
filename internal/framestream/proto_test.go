package framestream

import (
	"testing"

	pb "github.com/vsekhar/braid/pkg/api/braidpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestFrameSelfDelimiting(t *testing.T) {
	f := &pb.Frame{}
	fields := f.ProtoReflect().Descriptor().Fields()
	if fields.Len() == 0 {
		// technically valid if intentional, but highly unlikely to be.
		t.Error("empty frame message")
	}

	// Get the oneof descriptor from the first field.
	first := fields.Get(0)
	oneOf := first.ContainingOneof()
	if oneOf == nil || oneOf.IsPlaceholder() || oneOf.IsSynthetic() {
		t.Errorf("found field not in a oneof: %s", first.FullName())
	}

	// Ensure all other fields are part of the same oneof.
	var oneOfName string
	for i := 1; i < fields.Len(); i++ {
		field := fields.Get(i)
		o := field.ContainingOneof()

		// In a oneof.
		if o == nil {
			t.Errorf("found field not in a oneof: %s", field.FullName())
		}

		// In the *same* oneof.
		if oneOfName == "" {
			oneOfName = string(o.FullName())
		} else if string(o.FullName()) != oneOfName {
			t.Errorf("multiple oneofs found: %s and %s", o.FullName(), oneOf.FullName())
		}

		// Of kind message.
		if field.Kind() != protoreflect.MessageKind {
			t.Errorf("expected field of message type, got %s of type %s", field.FullName(), field.Kind().String())
		}
	}
}
