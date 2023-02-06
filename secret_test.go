package braid

import (
	"bytes"
	"testing"
)

func TestSecretsMustDiffer(t *testing.T) {
	i1, i2 := NewIdentity().(*secret), NewIdentity().(*secret)
	if bytes.Equal(i1.priv, i2.priv) {
		t.Error("expected different identities, got identical identities")
	}
}
