package braid

import (
	"context"
	"crypto/tls"
	"net"
)

type Transport struct {
	s   *secret
	cfg *tls.Config
}

func (t *Transport) Dial(network, address string) (net.Conn, error) {
	return t.DialContext(context.Background(), network, address)
}

func (t *Transport) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d := &tls.Dialer{Config: t.cfg}
	c, err := d.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (t *Transport) Listen(network, address string) (net.Listener, error) {
	return t.ListenContext(context.Background(), network, address)
}
func (t *Transport) ListenContext(ctx context.Context, network, address string) (net.Listener, error) {
	lcfg := &net.ListenConfig{}
	netl, err := lcfg.Listen(ctx, network, address)
	if err != nil {
		netl.Close()
		return nil, err
	}
	return tls.NewListener(netl, t.cfg), nil
}

func NewTransport(s Secret) *Transport {
	t := &Transport{
		s:   s.(*secret),
		cfg: initialTLSConfig.Clone(),
	}

	// Generate certificates dynamically to handle timeouts and
	// renegotiation by the TLS stack.
	t.cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return t.s.certificate(), nil
	}
	t.cfg.GetClientCertificate = func(req *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		// TODO: do I need to populate Certificate.Certificate?
		return t.s.certificate(), nil
	}
	return t
}
