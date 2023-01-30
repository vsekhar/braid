package peertls

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

type Conn struct {
	c tls.Conn
}

func (c *Conn) Read(b []byte) (n int, err error)   { return c.c.Read(b) }
func (c *Conn) Write(b []byte) (n int, err error)  { return c.c.Write(b) }
func (c *Conn) Close() error                       { return c.c.Close() }
func (c *Conn) LocalAddr() net.Addr                { return c.c.LocalAddr() }
func (c *Conn) RemoteAddr() net.Addr               { return c.c.RemoteAddr() }
func (c *Conn) SetDeadline(t time.Time) error      { return c.c.SetDeadline(t) }
func (c *Conn) SetReadDeadline(t time.Time) error  { return c.c.SetReadDeadline(t) }
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.c.SetWriteDeadline(t) }
func (c *Conn) Identity() Identity {
	// TODO: get private key from TLS connection and construct identity
	panic("unimplmeneted")
}

var _ net.Conn = (*Conn)(nil)

// Identity is a reference to a specific peer.
type Identity struct {
}

func DialIdentity(ctx context.Context, i *Identity)

type Host struct {
	// TODO: tls configs, etc.
}

func New(id []byte) *Host {
	return &Host{}
}

func (h *Host) Dial(network, address string) (net.Conn, error) {
	return h.DialContext(context.Background(), network, address)
}

func (h *Host) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	panic("unimplemented")
}

type listener struct {
	l net.Listener
	h *Host
}

func (l *listener) Accept() (net.Conn, error) {
	stdc, err := l.l.Accept()
	if err != nil {
		stdc.Close()
		return nil, err
	}

	// TODO: negotiate mtls
}

func (l *listener) Close() error {
	panic("unimplemented")
}

func (l *listener) Addr() net.Addr {
	panic("unimplemented")
}

var _ net.Listener = (*listener)(nil)

func (h *Host) Listen(network, address string) (net.Listener, error) {
	return h.ListenContext(context.Background(), network, address)
}

func (h *Host) ListenContext(ctx context.Context, network, address string) (net.Listener, error) {
	cfg := net.ListenConfig{}
	stdl, err := cfg.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return &listener{l: stdl, h: h}, nil
}
