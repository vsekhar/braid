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

const messageInterval = 500 * time.Millisecond
const gossipInterval = 7 * time.Second
const connectInterval = 3 * time.Second
const wantedInterval = 5 * time.Second
const targetConnections = 5
const targetConnectionsEpsilon = 2

// NodeConfig configures a Node.
type NodeConfig struct {
	ListenAddr     string
	Identity       *Identity
	AdmitPeer      AdmissionFunc
	BootstrapPeers []string
}

// Node is a braid protocol participant that listens for and dials peers.
type Node struct {
	cfg       NodeConfig
	listener  *Listener
	peers     *PeerSet       // currently connected peers
	directory *PeerDirectory // all known peers (connected or not)
	store     *Store         // message DAG
	selfID    string
	wg        sync.WaitGroup
	logger    *slog.Logger
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
	selfID := publicKeyID(cfg.Identity.PublicKey())[:8]
	return &Node{
		cfg:       cfg,
		listener:  ln,
		peers:     NewPeerSet(),
		directory: NewPeerDirectory(),
		store:     NewStore(),
		logger:    slog.Default().With("node", selfID),
		selfID:    selfID,
	}, nil
}

// ID returns the node's short hex identity string.
func (n *Node) ID() string {
	return n.selfID
}

// Addr returns the listener's address.
func (n *Node) Addr() net.Addr {
	return n.listener.Addr()
}

// Peers returns a snapshot of connected peers.
func (n *Node) Peers() []*Peer {
	return n.peers.All()
}

// Store returns the node's message store.
func (n *Node) Store() *Store {
	return n.store
}

// Directory returns the node's peer directory.
func (n *Node) Directory() *PeerDirectory {
	return n.directory
}

// Run starts accepting connections and connects to bootstrap peers.
// It blocks until ctx is canceled.
func (n *Node) Run(ctx context.Context) error {
	// Accept loop.
	n.wg.Go(func() {
		for {
			conn, err := n.listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				n.logger.Error("accept error", "err", err)
				continue
			}
			n.addPeer(conn, "")
		}
	})

	// Connect to bootstrap peers.
	for _, addr := range n.cfg.BootstrapPeers {
		n.wg.Go(func() {
			if err := n.Connect(ctx, addr); err != nil {
				n.logger.Error("bootstrap connect failed", "err", err)
			}
		})
	}

	// Gossip loop.
	n.wg.Go(func() {
		n.gossipLoop(ctx)
	})

	// Connection maintenance loop.
	n.wg.Go(func() {
		n.connectLoop(ctx)
	})

	// Wanted request loop.
	n.wg.Go(func() {
		n.wantedLoop(ctx)
	})

	// Message creation loop.
	n.wg.Go(func() {
		n.messageLoop(ctx)
	})

	<-ctx.Done()
	n.listener.Close()
	// Close all peer connections so read loops unblock.
	for _, p := range n.peers.All() {
		p.Conn.Close()
	}
	n.wg.Wait()
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
	n.addPeer(conn, addr)
	return nil
}

func (n *Node) addPeer(conn net.Conn, listenAddr string) {
	tlsConn := conn.(*tls.Conn)
	peerKey, err := PeerPublicKeyFromTLS(tlsConn)
	if err != nil {
		n.logger.Error("failed to extract peer key", "err", err)
		conn.Close()
		return
	}
	n.directory.Add(peerKey, listenAddr)
	n.directory.ResetErrors(peerKey)
	p := &Peer{
		Key:         peerKey,
		Conn:        conn,
		ConnectedAt: time.Now(),
		logger:      n.logger,
	}
	n.peers.Add(p)
	n.logger.Info("connected", "peer", publicKeyID(peerKey)[:8])

	// Start read loop for this peer.
	n.wg.Go(func() {
		n.readLoop(p)
	})
}

func (n *Node) readLoop(p *Peer) {
	defer func() {
		n.peers.Remove(p.Key)
		p.Conn.Close()
		n.logger.Info("disconnected from peer", "peer", publicKeyID(p.Key)[:8])
	}()
	for {
		env, err := ReadEnvelope(p.Conn)
		if err != nil {
			return
		}
		switch body := env.Body.(type) {
		case *Envelope_Message:
			n.handleMessage(body.Message)
		case *Envelope_PeerGossip:
			n.handleGossip(body.PeerGossip)
		case *Envelope_MessageRequest:
			n.handleMessageRequest(p, body.MessageRequest)
		}
	}
}

func (n *Node) handleMessage(msg *Message) {
	ref, isNew, err := n.store.Add(msg)
	if err != nil {
		n.logger.Error("failed to add message", "err", err)
		return
	}
	if !isNew {
		return
	}
	n.logger.Info("received message", "ref", refKey(ref)[:8],
		"incorporated", n.store.Len(), "pending", n.store.PendingLen(),
		"wanted", len(n.store.Wanted()))
}

func (n *Node) handleGossip(gossip *PeerGossip) {
	for _, pi := range gossip.GetPeers() {
		if publicKeyID(pi.Key) == publicKeyID(n.cfg.Identity.PublicKey()) {
			continue
		}
		if n.directory.Add(pi.Key, pi.GetAddress()) {
			n.logger.Info("learned peer", "peer", publicKeyID(pi.Key)[:8])
		}
	}
}

func (n *Node) handleMessageRequest(p *Peer, req *MessageRequest) {
	msgs := n.store.Resolve(req.Wanted, req.Frontier)
	if len(msgs) == 0 {
		return
	}
	n.logger.Info("resolving wanted", "peer", publicKeyID(p.Key)[:8], "sending", len(msgs))
	for _, msg := range msgs {
		env := &Envelope{
			Body: &Envelope_Message{Message: msg},
		}
		if err := p.Send(env); err != nil {
			return
		}
	}
}

func (n *Node) wantedLoop(ctx context.Context) {
	ticker := time.NewTicker(wantedInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.sendMessageRequest()
		}
	}
}

func (n *Node) sendMessageRequest() {
	wanted := n.store.Wanted()
	if len(wanted) == 0 {
		return
	}
	p := n.peers.Random()
	if p == nil {
		return
	}
	frontier := n.store.Frontier()
	env := &Envelope{
		Body: &Envelope_MessageRequest{
			MessageRequest: &MessageRequest{
				Wanted:   wanted,
				Frontier: frontier,
			},
		},
	}
	n.logger.Info("sending wanted request", "peer", publicKeyID(p.Key)[:8],
		"wanted", len(wanted), "frontier", len(frontier))
	p.Send(env)
}

func (n *Node) connectLoop(ctx context.Context) {
	ticker := time.NewTicker(connectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.maintainConnections(ctx)
		}
	}
}

func (n *Node) maintainConnections(ctx context.Context) {
	current := n.peers.Len()
	switch {
	case current > targetConnections+targetConnectionsEpsilon:
		// Drop a random connection.
		if p := n.peers.Random(); p != nil {
			n.logger.Info("dropping excess peer", "peer", publicKeyID(p.Key)[:8])
			p.Conn.Close() // read loop will clean up
		}
	case current < targetConnections-targetConnectionsEpsilon:
		// Connect to a random non-connected peer from the directory.
		kp := n.directory.RandomNotIn(n.peers)
		if kp == nil {
			return
		}
		if err := n.Connect(ctx, kp.Address); err != nil {
			n.directory.RecordError(kp.Key)
			n.logger.Error("connect failed", "peer", publicKeyID(kp.Key)[:8], "err", err)
		}
	}
}

func (n *Node) gossipLoop(ctx context.Context) {
	ticker := time.NewTicker(gossipInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.pushGossip()
		}
	}
}

func (n *Node) pushGossip() {
	p := n.peers.Random()
	if p == nil {
		return
	}
	infos := n.directory.PeerInfos()
	if len(infos) == 0 {
		return
	}
	env := &Envelope{
		Body: &Envelope_PeerGossip{
			PeerGossip: &PeerGossip{Peers: infos},
		},
	}
	p.Send(env)
}

func (n *Node) messageLoop(ctx context.Context) {
	ticker := time.NewTicker(messageInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.pushMessage()
		}
	}
}

func (n *Node) pushMessage() {
	msg, ref, err := n.store.CreateMessage(n.cfg.Identity)
	if err != nil {
		n.logger.Error("failed to create message", "err", err)
		return
	}
	env := &Envelope{
		Body: &Envelope_Message{Message: msg},
	}
	peers := n.peers.RandomN(5)
	for _, p := range peers {
		p.Send(env)
	}
	n.logger.Info("created message", "ref", refKey(ref)[:8],
		"peers", len(peers), "incorporated", n.store.Len())
}
