package braid

import (
	"encoding/hex"
	"net"
	"sync"
	"time"
)

// Peer represents a connected peer.
type Peer struct {
	Key         *PublicKey
	Conn        net.Conn
	ConnectedAt time.Time
}

// PeerSet is a thread-safe set of active peers keyed by public key.
type PeerSet struct {
	mu    sync.RWMutex
	peers map[string]*Peer // keyed by hex-encoded public key
}

// NewPeerSet creates an empty PeerSet.
func NewPeerSet() *PeerSet {
	return &PeerSet{peers: make(map[string]*Peer)}
}

// Add adds a peer to the set.
func (ps *PeerSet) Add(p *Peer) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.peers[publicKeyID(p.Key)] = p
}

// Remove removes a peer by public key.
func (ps *PeerSet) Remove(key *PublicKey) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.peers, publicKeyID(key))
}

// Get returns a peer by public key.
func (ps *PeerSet) Get(key *PublicKey) (*Peer, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	p, ok := ps.peers[publicKeyID(key)]
	return p, ok
}

// All returns a snapshot of all connected peers.
func (ps *PeerSet) All() []*Peer {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		out = append(out, p)
	}
	return out
}

// Len returns the number of connected peers.
func (ps *PeerSet) Len() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.peers)
}

func publicKeyID(pk *PublicKey) string {
	return hex.EncodeToString(pk.GetEd25519V1())
}
