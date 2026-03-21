package braid

import (
	"crypto/tls"
	"fmt"
	"net"
)

// AdmissionFunc decides whether to accept a connection from a peer
// identified by the given public key. Return nil to accept, or an
// error to reject.
type AdmissionFunc func(peerKey *PublicKey) error

// Listener is a net.Listener that performs mTLS handshakes and
// admission control before returning connections.
type Listener struct {
	inner net.Listener
	admit AdmissionFunc
}

// Listen creates a TLS listener with admission control. If admit is nil,
// all peers are accepted.
func Listen(network, address string, id *Identity, admit AdmissionFunc) (*Listener, error) {
	tlsCfg, err := id.ServerTLSConfig()
	if err != nil {
		return nil, err
	}
	ln, err := tls.Listen(network, address, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("listening: %w", err)
	}
	return &Listener{inner: ln, admit: admit}, nil
}

// Accept waits for and returns the next admitted connection. Connections
// that fail the TLS handshake or are rejected by the AdmissionFunc are
// closed and skipped.
func (l *Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.inner.Accept()
		if err != nil {
			return nil, err
		}
		tlsConn := conn.(*tls.Conn)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			continue
		}
		peerKey, err := PeerPublicKeyFromTLS(tlsConn)
		if err != nil {
			conn.Close()
			continue
		}
		if l.admit != nil {
			if err := l.admit(peerKey); err != nil {
				conn.Close()
				continue
			}
		}
		return conn, nil
	}
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.inner.Addr()
}

// Close closes the listener.
func (l *Listener) Close() error {
	return l.inner.Close()
}
