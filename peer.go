package braid

import (
	"encoding/hex"
	"math/rand/v2"
	"net"
	"sync"
	"time"
)

// Peer represents a connected peer.
type Peer struct {
	Key         *PublicKey
	Conn        net.Conn
	ConnectedAt time.Time
	sendCh      chan *Envelope

	// sharedFrontier tracks refs known to be on both sides of this
	// connection. It is only accessed from the peer's readLoop goroutine.
	sharedFrontier map[string]struct{}
}

// advanceSharedFrontier adds a ref to the shared frontier and removes any of
// its parents that were previously in the frontier. This keeps the shared
// frontier minimal — only the tips of the known-shared region.
func (p *Peer) advanceSharedFrontier(key string, msg *Message) {
	p.sharedFrontier[key] = struct{}{}
	for _, entry := range msg.GetParents().GetEntries() {
		delete(p.sharedFrontier, refKey(entry.GetParent()))
	}
}

// Enqueue adds an envelope to the peer's send queue. Returns false if the
// queue is full (caller should log/drop).
func (p *Peer) Enqueue(env *Envelope) bool {
	select {
	case p.sendCh <- env:
		return true
	default:
		return false
	}
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

// Random returns a random peer, or nil if the set is empty.
func (ps *PeerSet) Random() *Peer {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if len(ps.peers) == 0 {
		return nil
	}
	i := rand.IntN(len(ps.peers))
	for _, p := range ps.peers {
		if i == 0 {
			return p
		}
		i--
	}
	return nil
}

// RandomN returns up to n random peers from the set.
func (ps *PeerSet) RandomN(n int) []*Peer {
	all := ps.All()
	if len(all) <= n {
		return all
	}
	rand.Shuffle(len(all), func(i, j int) {
		all[i], all[j] = all[j], all[i]
	})
	return all[:n]
}

func publicKeyID(pk *PublicKey) string {
	return hex.EncodeToString(pk.GetEd25519V1())
}
