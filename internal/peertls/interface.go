package peertls

import (
	"encoding"
)

// Identity is a public reference to a specific peer.
//
// Identity is conceptually similar to a cryptographic public key and can be
// shared freely.
//
// Identity is not validated or signed by any other authority and it is up to
// the application to determine if the peer presenting an Identity is
// trustworthy.
type Identity interface {
	marshalable

	// Equals returns true if and only if the Identities are equal.
	Equals(q Identity) bool
}

// Secret is a private value used to act as an Identity.
//
// Secret is conceptually similar to a cryptographic private key and should not
// be shared.
type Secret interface {
	marshalable

	Identity() Identity
	Equals(s Secret) bool
}

type marshalable interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	// This package uses protobufs under the hood and proto text encodings are
	// not stable, so we use Debug* methods rather than encoding.TextMarshaler
	// and encoding.TextUnmarshaler to help prevent accidental reliance on
	// text encodings for canonical serialization.

	DebugMarshalText() (text []byte, err error)
	DebugUnmarshalText(text []byte) error
}
