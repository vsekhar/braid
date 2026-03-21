package braid

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// NodeConfig configures a Node.
type NodeConfig struct {
	ListenAddr     string
	Identity       *Identity
	AdmitPeer      AdmissionFunc
	BootstrapPeers []string
}

// Node is a braid protocol participant that listens for and dials peers.
type Node struct {
	cfg      NodeConfig
	listener *Listener
	peers    *PeerSet
	logger   *slog.Logger
}

// NewNode creates a Node. Call Run to start it.
func NewNode(cfg NodeConfig) (*Node, error) {
	if cfg.Identity == nil {
		return nil, fmt.Errorf("identity is required")
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8443"
	}
	ln, err := Listen("tcp", cfg.ListenAddr, cfg.Identity, cfg.AdmitPeer)
	if err != nil {
		return nil, err
	}
	return &Node{
		cfg:      cfg,
		listener: ln,
		peers:    NewPeerSet(),
		logger:   slog.Default(),
	}, nil
}

// Addr returns the listener's address.
func (n *Node) Addr() net.Addr {
	return n.listener.Addr()
}

// Peers returns a snapshot of connected peers.
func (n *Node) Peers() []*Peer {
	return n.peers.All()
}

// Run starts accepting connections and connects to bootstrap peers.
// It blocks until ctx is canceled.
func (n *Node) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	// Accept loop.
	wg.Go(func() {
		for {
			conn, err := n.listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				n.logger.Error("accept error", "err", err)
				continue
			}
			n.addPeer(conn)
		}
	})

	// Connect to bootstrap peers.
	for _, addr := range n.cfg.BootstrapPeers {
		wg.Go(func() {
			if err := n.Connect(ctx, addr); err != nil {
				n.logger.Error("bootstrap connect failed", "addr", addr, "err", err)
			}
		})
	}

	// Wait for context cancellation.
	<-ctx.Done()
	n.listener.Close()
	wg.Wait()
	return ctx.Err()
}

// Connect dials a peer and adds it to the peer set.
func (n *Node) Connect(ctx context.Context, addr string) error {
	tlsCfg, err := n.cfg.Identity.ClientTLSConfig()
	if err != nil {
		return err
	}
	dialer := &tls.Dialer{Config: tlsCfg}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dialing %s: %w", addr, err)
	}
	n.addPeer(conn)
	return nil
}

func (n *Node) addPeer(conn net.Conn) {
	tlsConn := conn.(*tls.Conn)
	peerKey, err := PeerPublicKeyFromTLS(tlsConn)
	if err != nil {
		n.logger.Error("failed to extract peer key", "err", err)
		conn.Close()
		return
	}
	p := &Peer{
		Key:         peerKey,
		Conn:        conn,
		ConnectedAt: time.Now(),
	}
	n.peers.Add(p)
	n.logger.Info("peer connected", "key", publicKeyID(peerKey), "addr", conn.RemoteAddr())
}
