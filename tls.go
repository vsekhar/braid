package braid

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

// TLSCertificate generates a self-signed X.509 certificate from the identity's
// ed25519 key. The certificate is used only to carry the public key during
// TLS handshakes; trust is based on braid reputation, not CA chains.
func (id *Identity) TLSCertificate() (tls.Certificate, error) {
	pub := id.key.Public().(ed25519.PublicKey)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               newPkixName(pub),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, id.key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("creating self-signed certificate: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  id.key,
	}, nil
}

// ServerTLSConfig returns a tls.Config for accepting mTLS connections.
func (id *Identity) ServerTLSConfig() (*tls.Config, error) {
	cert, err := id.TLSCertificate()
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAnyClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// ClientTLSConfig returns a tls.Config for dialing mTLS connections.
func (id *Identity) ClientTLSConfig() (*tls.Config, error) {
	cert, err := id.TLSCertificate()
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // trust is braid-reputation-based, not CA-based
		MinVersion:         tls.VersionTLS13,
	}, nil
}

// PeerPublicKeyFromTLS extracts the peer's ed25519 public key from a TLS
// connection's peer certificate.
func PeerPublicKeyFromTLS(conn *tls.Conn) (*PublicKey, error) {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, fmt.Errorf("peer presented no certificate")
	}
	pub, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("peer certificate key is %T, not ed25519", state.PeerCertificates[0].PublicKey)
	}
	return &PublicKey{Ed25519V1: []byte(pub)}, nil
}

func newPkixName(pub ed25519.PublicKey) pkix.Name {
	return pkix.Name{CommonName: hex.EncodeToString(pub)}
}
