package peertls

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"time"

	"github.com/vsekhar/braid/internal/peertls/internal/peertlspb"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type identity struct {
	pub ed25519.PublicKey
}

var _ Identity = (*identity)(nil)

func (i *identity) Equals(j Identity) bool {
	return bytes.Equal(i.pub, j.(*identity).pub)
}

func (i *identity) toPb() *peertlspb.Identity {
	return &peertlspb.Identity{
		Ed25519PublicKey: append([]byte(nil), i.pub...),
	}
}

func (i *identity) fromPb(pb *peertlspb.Identity) {
	i.pub = append(i.pub[:0], pb.Ed25519PublicKey...)
}

type unmarshalFunc func([]byte, protoreflect.ProtoMessage) error

func (i *identity) unmarshal(b []byte, u unmarshalFunc) error {
	pb := &peertlspb.Identity{}
	if err := u(b, pb); err != nil {
		return err
	}
	i.fromPb(pb)
	return nil
}

func (i *identity) MarshalBinary() ([]byte, error)    { return proto.Marshal(i.toPb()) }
func (i *identity) UnmarshalBinary(b []byte) error    { return i.unmarshal(b, proto.Unmarshal) }
func (i *identity) DebugMarshalText() ([]byte, error) { return prototext.Marshal(i.toPb()) }
func (i *identity) DebugUnmarshalText(b []byte) error { return i.unmarshal(b, prototext.Unmarshal) }

type secret struct {
	priv ed25519.PrivateKey
}

var _ Secret = (*secret)(nil)

func (s *secret) Equals(t Secret) bool {
	return bytes.Equal(s.priv, t.(*secret).priv)
}

func (s *secret) toPb() *peertlspb.Secret {
	return &peertlspb.Secret{
		Ed25519PrivateKey: append([]byte(nil), s.priv...),
	}
}

func (s *secret) fromPb(pb *peertlspb.Secret) {
	s.priv = append(s.priv[:0], pb.Ed25519PrivateKey...)
}

func (s *secret) unmarshal(b []byte, u unmarshalFunc) error {
	pb := &peertlspb.Secret{}
	if err := u(b, pb); err != nil {
		return err
	}
	s.fromPb(pb)
	return nil
}

func (s *secret) MarshalBinary() ([]byte, error)    { return proto.Marshal(s.toPb()) }
func (s *secret) UnmarshalBinary(b []byte) error    { return s.unmarshal(b, proto.Unmarshal) }
func (s *secret) DebugMarshalText() ([]byte, error) { return prototext.Marshal(s.toPb()) }
func (s *secret) DebugUnmarshalText(b []byte) error { return s.unmarshal(b, prototext.Unmarshal) }

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

func NewIdentity() Secret {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	return &secret{priv}
}
