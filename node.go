package braid

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

const messageInterval = 500 * time.Millisecond
const gossipInterval = 7 * time.Second
const connectInterval = 3 * time.Second
const probeInterval = 2 * time.Second
const targetConnections = 8
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
			n.addPeer(ctx, conn, "")
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

	// Probe-based synchronization loop.
	n.wg.Go(func() {
		n.probeLoop(ctx)
	})

	// Message creation loop.
	n.wg.Go(func() {
		n.messageLoop(ctx)
	})

	<-ctx.Done()
	n.listener.Close()
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
	n.addPeer(ctx, conn, addr)
	return nil
}

func (n *Node) addPeer(ctx context.Context, conn net.Conn, listenAddr string) {
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
		sendCh:      make(chan *Envelope, 8192),
	}
	n.peers.Add(p)
	n.logger.Info("connected", "peer", publicKeyID(peerKey)[:8])

	// Start read and write loops for this peer.
	n.wg.Go(func() {
		n.readLoop(ctx, p)
	})
	n.wg.Go(func() {
		n.writeLoop(ctx, p)
	})
}

func (n *Node) writeLoop(ctx context.Context, p *Peer) {
	stop := context.AfterFunc(ctx, func() { p.Conn.Close() })
	defer stop()
	for env := range p.sendCh {
		if err := WriteEnvelope(p.Conn, env); err != nil {
			p.Conn.Close() // unblocks readLoop
			for range p.sendCh {
			}
			return
		}
	}
}

func (n *Node) readLoop(ctx context.Context, p *Peer) {
	// Close the connection when the context is cancelled, unblocking ReadEnvelope.
	stop := context.AfterFunc(ctx, func() { p.Conn.Close() })
	defer stop()
	defer func() {
		n.peers.Remove(p.Key)
		p.Conn.Close()
		close(p.sendCh) // signal writeLoop to exit
		n.logger.Info("disconnected from peer", "peer", publicKeyID(p.Key)[:8])
	}()
	for {
		env, err := ReadEnvelope(p.Conn)
		if err != nil {
			return
		}
		switch body := env.Body.(type) {
		case *Envelope_Message:
			n.handleMessage(p, body.Message)
		case *Envelope_PeerGossip:
			n.handleGossip(body.PeerGossip)
		case *Envelope_ProbeRequest:
			n.handleProbeRequest(p, body.ProbeRequest)
		case *Envelope_ProbeResponse:
			n.handleProbeResponse(p, body.ProbeResponse)
		default:
			unknown := env.ProtoReflect().GetUnknown()
			num, typ, length := protowire.ConsumeField(unknown)
			n.logger.Info("dropping envelope with unknown contents", "peer", publicKeyID(p.Key)[:8],
				"field", num, "wire_type", typ, "length", length)
		}
	}
}

func (n *Node) handleMessage(p *Peer, msg *Message) {
	result, err := n.store.Add(msg)
	if err != nil {
		n.logger.Error("failed to add message", "err", err)
		return
	}
	if !result.IsNew {
		n.logger.Info("duplicate message", "ref", refKey(result.Ref)[:8],
			"peer", publicKeyID(p.Key)[:8])
		return
	}
	n.logger.Info("received message", "ref", refKey(result.Ref)[:8],
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

func (n *Node) handleProbeRequest(p *Peer, req *ProbeRequest) {
	// Classify each probed ref. Use Get (incorporated only) so the
	// boundary detection treats pending messages as missing.
	var have, want []*MessageRef
	for _, ref := range req.GetHave() {
		if _, ok := n.store.Get(ref); ok {
			have = append(have, ref)
		} else {
			want = append(want, ref)
		}
	}
	if len(have) == 0 && len(want) == 0 {
		return
	}
	p.Enqueue(&Envelope{
		Body: &Envelope_ProbeResponse{
			ProbeResponse: &ProbeResponse{Have: have, Want: want},
		},
	})
}

func (n *Node) handleProbeResponse(p *Peer, resp *ProbeResponse) {
	// Initialize DAGdiff state on first response for this probe round.
	if p.proposals == nil {
		p.proposals = make(map[string]dagdiffProposal)
		p.hits = make(map[string]struct{})
		p.misses = make(map[string]struct{})
		p.boundary = make(map[string]struct{})
		for _, ref := range resp.GetHave() {
			p.proposals[refKey(ref)] = dagdiffProposal{direction: -1, magnitude: 1, rng: -1}
		}
		for _, ref := range resp.GetWant() {
			p.proposals[refKey(ref)] = dagdiffProposal{direction: -1, magnitude: 1, rng: -1}
		}
	}

	// Cache responses.
	for _, ref := range resp.GetHave() {
		k := refKey(ref)
		p.hits[k] = struct{}{}
		delete(p.misses, k)
	}
	for _, ref := range resp.GetWant() {
		k := refKey(ref)
		p.misses[k] = struct{}{}
		delete(p.hits, k)
	}

	// Update all proposals against the cache.
	newProposals := make(map[string]dagdiffProposal)
	for k, prop := range p.proposals {
		n.processProposal(p, k, prop.direction, prop.magnitude, prop.rng, newProposals)
	}
	p.proposals = newProposals

	if len(p.proposals) == 0 {
		// Converged — forward walk from boundary (inclusive) = delta.
		if len(p.boundary) > 0 {
			msgs := n.store.ForwardWalkInclusive(p.boundary)
			for _, msg := range msgs {
				ref, _ := HashMessage(msg)
				k := refKey(ref)
				if !p.Enqueue(&Envelope{Body: &Envelope_Message{Message: msg}}) {
					break
				}
				// Mark sent refs as hits so re-probes don't re-send
				// messages still in the TCP pipeline (#4).
				p.hits[k] = struct{}{}
				delete(p.misses, k)
			}
			if len(msgs) > 0 {
				n.logger.Info("probe: sending delta", "peer", publicKeyID(p.Key)[:8],
					"boundary", len(p.boundary), "sent", len(msgs))
			}
		}
		// If we sent a delta, immediately start a new round to chase
		// convergence at network speed (#1). Keep hits/misses cache
		// across rounds so sent-but-undelivered refs are treated as
		// shared (#4). If boundary was empty (peer is caught up),
		// go idle and let probeLoop start the next cycle.
		p.proposals = nil
		if len(p.boundary) > 0 {
			p.boundary = nil
			n.startProbe(p)
		} else {
			p.boundary = nil
		}
	} else {
		// Send next ProbeRequest with remaining proposals.
		refs := make([]*MessageRef, 0, len(p.proposals))
		for k := range p.proposals {
			b, _ := hex.DecodeString(k)
			refs = append(refs, &MessageRef{Sha256V1: b})
		}
		p.Enqueue(&Envelope{
			Body: &Envelope_ProbeRequest{
				ProbeRequest: &ProbeRequest{Have: refs},
			},
		})
	}
}

// processProposal updates a single DAGdiff proposal against the cached
// hits/misses. It may recurse for new proposals that have cached responses.
func (n *Node) processProposal(p *Peer, ref string, direction, magnitude, rng int, out map[string]dagdiffProposal) {
	_, isHit := p.hits[ref]
	_, isMiss := p.misses[ref]

	if !isHit && !isMiss {
		// No response yet — carry forward unchanged.
		out[ref] = dagdiffProposal{direction, magnitude, rng}
		return
	}

	respDir := 1
	if isMiss {
		respDir = -1
	}

	var nextMag, nextRng int
	if rng < 0 {
		// Expansion phase.
		if direction == respDir {
			nextMag = magnitude * 2
			nextRng = -1
		} else {
			nextRng = magnitude
			nextMag = magnitude / 2
		}
	} else {
		// Narrowing phase.
		nextMag = rng / 2
		nextRng = nextMag
	}

	if nextMag == 0 {
		// Converged.
		if isMiss {
			p.boundary[ref] = struct{}{}
		} else {
			// Correction: probe children to find exact boundary.
			for _, childKey := range n.store.Walk(ref, 1) {
				if _, ok := p.boundary[childKey]; !ok {
					if _, ok := out[childKey]; !ok {
						n.processProposal(p, childKey, 1, 1, 1, out)
					}
				}
			}
		}
	} else {
		// Walk to next position; shorten hop if it goes off the DAG.
		walkMag := nextMag
		var reached []string
		for walkMag > 0 {
			reached = n.store.Walk(ref, respDir*walkMag)
			if len(reached) > 0 {
				break
			}
			walkMag /= 2
			if nextRng >= 0 {
				nextRng = walkMag
			}
		}
		if len(reached) > 0 {
			for _, r := range reached {
				if _, ok := p.boundary[r]; !ok {
					if _, ok := out[r]; !ok {
						n.processProposal(p, r, respDir, walkMag, nextRng, out)
					}
				}
			}
		} else if isMiss {
			p.boundary[ref] = struct{}{}
		}
	}
}

func (n *Node) probeLoop(ctx context.Context) {
	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.sendProbe()
		}
	}
}

func (n *Node) sendProbe() {
	frontier := n.store.Frontier()
	if len(frontier) == 0 {
		return
	}
	for _, p := range n.peers.All() {
		// probeLoop is the fresh-start path: clear the cache so stale
		// hits don't suppress needed sends.
		p.hits = nil
		p.misses = nil
		p.proposals = nil
		p.boundary = nil
		n.startProbe(p)
	}
}

// startProbe sends a ProbeRequest with the current frontier for a single peer.
func (n *Node) startProbe(p *Peer) {
	frontier := n.store.Frontier()
	if len(frontier) == 0 {
		return
	}
	env := &Envelope{
		Body: &Envelope_ProbeRequest{
			ProbeRequest: &ProbeRequest{
				Have: frontier,
			},
		},
	}
	p.Enqueue(env)
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
	p.Enqueue(env)
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
		p.Enqueue(env)
	}
	n.logger.Info("created message", "ref", refKey(ref)[:8],
		"peers", len(peers), "incorporated", n.store.Len())
}
