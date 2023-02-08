// Package ed25519 is intended to force the use of ZIP215's precise validation
// criteria for Ed25519. Use this library instead of crypto/ed25519.
//
// This package is intentionally named the same as crypto/ed25519.
package ed25519

import (
	stdEd25519 "crypto/ed25519"
	"io"

	"github.com/hdevalence/ed25519consensus"
)

type (
	PublicKey  = stdEd25519.PublicKey
	PrivateKey = stdEd25519.PrivateKey
)

const (
	PublicKeySize  = stdEd25519.PublicKeySize
	PrivateKeySize = stdEd25519.PrivateKeySize
	SignatureSize  = stdEd25519.SignatureSize
	SeedSize       = stdEd25519.SeedSize
)

func Sign(p PrivateKey, m []byte) []byte                     { return stdEd25519.Sign(p, m) }
func GenerateKey(r io.Reader) (PublicKey, PrivateKey, error) { return stdEd25519.GenerateKey(r) }
func NewKeyFromSeed(s []byte) PrivateKey                     { return stdEd25519.NewKeyFromSeed(s) }

func Verify(publicKey PublicKey, message, sig []byte) bool {
	// Use ZIP215-compatible Verify function instead of the standard library's.
	return ed25519consensus.Verify(publicKey, message, sig)
}
