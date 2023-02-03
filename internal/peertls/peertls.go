package peertls

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"
)

const certTimeout = 72 * time.Hour

var initialTLSConfig = &tls.Config{
	// Require a cert, don't verify the cert chain
	ClientAuth:         tls.RequireAnyClientCert,
	InsecureSkipVerify: true,
	VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) != 1 || verifiedChains != nil {
			return fmt.Errorf("bad certs: raw: %v; verified %v", rawCerts, verifiedChains)
		}
		x509Cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return err
		}
		if err := verifyX509Cert(x509Cert); err != nil {
			return err
		}
		return nil
	},
	RootCAs:    x509.NewCertPool(), // don't use host's CA cert pool
	MinVersion: tls.VersionTLS13,
}

type Host struct {
	i   *identity
	s   *secret
	cfg *tls.Config
}

func (h *Host) Dial(network, address string) (net.Conn, error) {
	return h.DialContext(context.Background(), network, address)
}

func (h *Host) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d := &tls.Dialer{Config: h.cfg}
	c, err := d.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (h *Host) Listen(network, address string) (net.Listener, error) {
	return h.ListenContext(context.Background(), network, address)
}
func (h *Host) ListenContext(ctx context.Context, network, address string) (net.Listener, error) {
	lcfg := &net.ListenConfig{}
	netl, err := lcfg.Listen(ctx, network, address)
	if err != nil {
		netl.Close()
		return nil, err
	}
	return tls.NewListener(netl, h.cfg), nil
}

func verifyX509Cert(cert *x509.Certificate) error {
	if _, ok := cert.PublicKey.(ed25519.PublicKey); !ok {
		return fmt.Errorf("expected ed25519.PublicKey, got %T", cert.PublicKey)
	}
	if cert.PublicKeyAlgorithm != x509.Ed25519 {
		return fmt.Errorf("expected public key algorithm x509.Ed25519, got %s", cert.PublicKeyAlgorithm.String())
	}
	if cert.SignatureAlgorithm != x509.PureEd25519 {
		return fmt.Errorf("expected signature algorithm x509.PureEd25519, got %s", cert.SignatureAlgorithm.String())
	}
	if time.Now().Before(cert.NotBefore) {
		return fmt.Errorf("cert not yet valid (NotBefore %s)", cert.NotBefore)
	}
	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("cert expired (NotAfter %s)", cert.NotAfter)
	}
	return nil
}

func RemoteIdentity(c net.Conn) (Identity, error) {
	tlsConn, ok := c.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("expected TLS connection, got %T", c)
	}

	state := tlsConn.ConnectionState()
	if !state.HandshakeComplete {
		return nil, fmt.Errorf("handshake not complete")
	}
	if len(state.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no peer certificates")
	}
	if len(state.PeerCertificates) > 1 {
		return nil, fmt.Errorf("multiple peer certificates")
	}

	cert := state.PeerCertificates[0]
	if err := verifyX509Cert(cert); err != nil {
		return nil, err
	}

	return &identity{pub: cert.PublicKey.(ed25519.PublicKey)}, nil
}

func NewHost(s Secret) *Host {
	h := &Host{
		i:   s.Identity().(*identity),
		s:   s.(*secret),
		cfg: initialTLSConfig.Clone(),
	}

	// Generate certificates dynamically to handle timeouts and to support
	// renegotiation in the TLS stack.
	h.cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return h.s.certificate(), nil
	}
	h.cfg.GetClientCertificate = func(req *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		// TODO: do I need to populate Certificate.Certificate?
		return h.s.certificate(), nil
	}
	return h
}
