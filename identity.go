package braid

import (
	"bytes"
	"crypto/ed25519"
)

type identity struct {
	pub ed25519.PublicKey
}

var _ Identity = (*identity)(nil)

func (i *identity) Equals(j Identity) bool {
	return bytes.Equal(i.pub, j.(*identity).pub)
}

func NewIdentity() Secret {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	return &secret{priv}
}
