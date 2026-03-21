package braid

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// Identity holds an ed25519 private key used for signing messages and
// authenticating TLS connections.
type Identity struct {
	key ed25519.PrivateKey
}

// GenerateIdentity creates a new random ed25519 identity.
func GenerateIdentity() (*Identity, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ed25519 key: %w", err)
	}
	return &Identity{key: priv}, nil
}

// LoadIdentity reads a PEM-encoded PKCS#8 ed25519 private key from path.
func LoadIdentity(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is %T, not ed25519", parsed)
	}
	return &Identity{key: key}, nil
}

// Save writes the private key to path as PEM-encoded PKCS#8.
func (id *Identity) Save(path string) error {
	der, err := x509.MarshalPKCS8PrivateKey(id.key)
	if err != nil {
		return fmt.Errorf("marshaling private key: %w", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0600)
}

// PublicKey returns the protobuf PublicKey for this identity.
func (id *Identity) PublicKey() *PublicKey {
	pub := id.key.Public().(ed25519.PublicKey)
	return &PublicKey{Ed25519V1: []byte(pub)}
}

// Sign signs data and returns a protobuf Signature.
func (id *Identity) Sign(data []byte) *Signature {
	sig := ed25519.Sign(id.key, data)
	return &Signature{Ed25519V1: sig}
}

// VerifySignature verifies a signature against a public key and data.
func VerifySignature(pub *PublicKey, data []byte, sig *Signature) bool {
	pubBytes := pub.GetEd25519V1()
	sigBytes := sig.GetEd25519V1()
	if pubBytes == nil || sigBytes == nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubBytes), data, sigBytes)
}
