package braid

import (
	"math/rand/v2"
	"sync"
	"time"
)

// KnownPeer is a peer the node is aware of, whether connected or not.
type KnownPeer struct {
	Key       *PublicKey
	Address   string
	FirstSeen time.Time
	LastSeen  time.Time
	Errors    int // consecutive failed connection attempts
}

// PeerDirectory is a thread-safe directory of all known peers, keyed by
// public key identity. This includes peers we are connected to, peers
// learned via gossip, and peers we have been told about but never contacted.
type PeerDirectory struct {
	mu    sync.RWMutex
	peers map[string]*KnownPeer // keyed by publicKeyID
}

// NewPeerDirectory creates an empty PeerDirectory.
func NewPeerDirectory() *PeerDirectory {
	return &PeerDirectory{peers: make(map[string]*KnownPeer)}
}

// Add adds or updates a peer in the directory. Returns true if the peer
// was previously unknown.
func (d *PeerDirectory) Add(key *PublicKey, address string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := publicKeyID(key)
	if kp, ok := d.peers[id]; ok {
		kp.LastSeen = time.Now()
		if address != "" {
			kp.Address = address
		}
		return false
	}
	now := time.Now()
	d.peers[id] = &KnownPeer{
		Key:       key,
		Address:   address,
		FirstSeen: now,
		LastSeen:  now,
	}
	return true
}

// Get returns a known peer by public key.
func (d *PeerDirectory) Get(key *PublicKey) (*KnownPeer, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	kp, ok := d.peers[publicKeyID(key)]
	return kp, ok
}

// All returns a snapshot of all known peers.
func (d *PeerDirectory) All() []*KnownPeer {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*KnownPeer, 0, len(d.peers))
	for _, kp := range d.peers {
		out = append(out, kp)
	}
	return out
}

// Len returns the number of known peers.
func (d *PeerDirectory) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.peers)
}

// RecordError increments the consecutive error count for a peer.
func (d *PeerDirectory) RecordError(key *PublicKey) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if kp, ok := d.peers[publicKeyID(key)]; ok {
		kp.Errors++
	}
}

// ResetErrors sets the consecutive error count to zero for a peer.
func (d *PeerDirectory) ResetErrors(key *PublicKey) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if kp, ok := d.peers[publicKeyID(key)]; ok {
		kp.Errors = 0
	}
}

// RandomNotIn returns a random known peer whose key is not in the given
// PeerSet and that has a non-empty address. Returns nil if none available.
func (d *PeerDirectory) RandomNotIn(connected *PeerSet) *KnownPeer {
	d.mu.RLock()
	defer d.mu.RUnlock()
	candidates := make([]*KnownPeer, 0)
	for _, kp := range d.peers {
		if kp.Address == "" {
			continue
		}
		if _, ok := connected.Get(kp.Key); ok {
			continue
		}
		candidates = append(candidates, kp)
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[rand.IntN(len(candidates))]
}

// PeerInfos returns the directory contents as a slice of PeerInfo protos,
// suitable for gossip.
func (d *PeerDirectory) PeerInfos() []*PeerInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*PeerInfo, 0, len(d.peers))
	for _, kp := range d.peers {
		if kp.Address == "" {
			continue
		}
		addr := kp.Address
		out = append(out, &PeerInfo{
			Key:     kp.Key,
			Address: &addr,
		})
	}
	return out
}
