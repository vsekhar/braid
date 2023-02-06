package braid

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"time"
)

const certTimeout = 72 * time.Hour

type secret struct {
	priv ed25519.PrivateKey
}

var _ Secret = (*secret)(nil)

func (s *secret) pub() ed25519.PublicKey {
	return s.priv.Public().(ed25519.PublicKey)
}
func (s *secret) Identity() Identity {
	return &identity{pub: s.pub()}
}

func (s *secret) certificate() *tls.Certificate {
	x509Tmpl := &x509.Certificate{
		SerialNumber:       big.NewInt(2023),
		IsCA:               false,
		SignatureAlgorithm: x509.PureEd25519,
		NotBefore:          time.Now(),
		NotAfter:           time.Now().Add(certTimeout),
	}
	x509CertDer, err := x509.CreateCertificate(rand.Reader, x509Tmpl, x509Tmpl, s.pub(), s.priv)
	if err != nil {
		panic(err)
	}
	x509Cert, err := x509.ParseCertificate(x509CertDer)
	if err != nil {
		panic(err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{x509CertDer},
		PrivateKey:  s.priv,
		Leaf:        x509Cert,
	}
}
