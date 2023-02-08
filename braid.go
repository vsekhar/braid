package braid

import (
	stdbytes "bytes"
	"sort"
	"time"

	"github.com/vsekhar/braid/internal/bytes"
	"github.com/vsekhar/braid/internal/locked"
	"github.com/vsekhar/braid/pkg/ed25519"
)

const pendingMessageTimeout = 60 * time.Second

// frontierEntry is a wrapper for Parent, tracking the coveringSet for that
// entry on the frontier.
type frontierEntry struct {
	p *Parent // don't embed struct, may prevent frontierEntry from being GC'd

	// coveringSet contains all incremental messages reachable from this
	// frontier entry and not reachable from any higher frontier entry. The size
	// of this set is equal to Parent.relContrib for this entry.
	//
	// We retain the coveringSet of messages on the frontier so that we can
	// incrementally compute the relContrib of successive entries.
	coveringSet bytes.Map[*Message] // m.self --> *Message
}

// Orphan is a wrapper for a Message, tracking information about missing parents
// or missing parents of parents.
//
// Orphans form their own DAG with forward pointers and cumulative lists of
// missing parents. Orphans are referenced in Braid.missingParents, and by other
// orphans in orphan.children.
//
// Orphans are updated when a missing parent arrives. Updates propagate forward
// transitively to an orphan's children. When an orphan has neither missing
// parents nor transitive missing parents, it is no longer an orphan and the
// orphan struct is garbage collected. The underlying Message struct remains in
// the main Braid DAG.
type orphan struct {
	*Message
	parentRefs                  [][]byte // same order as Message.parents
	transitiveMissingParentRefs bytes.Set
	children                    []*orphan
}

// Braid is a data structure for asynchronously ordering a set of messages.
//
// The methods of Braid must not be called concurrently. The zero Braid is a
// valid empty Braid.
type Braid struct {
	horizon  locked.Value[[]orphan] // "blessed" orphans
	frontier locked.Value[[]frontierEntry]

	orphansByRef   bytes.Map[*orphan]   // m.self --> *orphan
	missingParents bytes.Map[[]*orphan] // parent's Ref.Shake256_64_V1 --> orphan
}

func (b *Braid) Has(h []byte) bool {
	for _, fe := range b.frontier.Get() {
		if fe.coveringSet.Has(h) {
			return true
		}
	}
	return false
}

// TODO: serialize a braid: len, frontier, messages, horizon

// Adding a message to a braid:

func (b *Braid) Len() int {
	var l uint64 = 0
	for _, fe := range b.frontier.Get() {
		l += fe.p.relContrib
	}
	return int(l)
}

// Cut returns the latest set of messages whose timestamps are less than or
// equal to t.
func (b *Braid) Cut(t time.Time) []*Message {
	s := make(map[string]*Message)
	f := b.frontier.Get()
	var p Parent
	for len(f) > 0 {
		p, f = f[len(f)-1], f[:len(f)-1]
		if p.msg.timestamp.After(t) {
			for _, p := range p.msg.parents {
				f = append(f, p)
			}
		} else {
			s[p.msg.self] = p.msg
		}
	}
	r := make([]*Message, 0, len(s))
	for _, m := range s {
		r = append(r, m)
	}
	return r
}

func (b *Braid) WriteAndSign(k ed25519.PrivateKey, d []byte) (int, error) {
	m := &Message{
		author: Authorship{
			publicKey: k.Public().(ed25519.PublicKey),
		},
		timestamp:       time.Now(),
		applicationData: d,
	}
	for _, p := range b.frontier.Get() {
		m.parents = append(m.parents, p)
	}

	// TODO: timestamp proof

	m.self = Ref(m)
	m.author.signature = ed25519.Sign(k, m.self)
	b.Add(m)
	return len(d), nil
}

func (b *Braid) addToFronter(m *Message) {
	var contribution uint64
	for _, p := range m.parents {
		contribution += p.contribution
	}
	p := cutMsg{}
	p.contribution = uint64(contribution + 1)
	p.refOrMsg.Set2(m)
	// TODO: use b.frontier.Update()
	i := sort.Search(len(b.frontier), func(i int) bool {
		if b.frontier[i].contribution > p.contribution {
			return false
		}
		if b.frontier[i].contribution == p.contribution {
			cmp := bytes.Compare(b.frontier[i].ref, m.self)
			if cmp == 0 {
				panic("duplicate message on frontier")
			}
			return cmp == 1
		}
		return true
	})
	b.frontier = append(b.frontier, cutMsg{})
	copy(b.frontier[i+1:], b.frontier[i:])
	b.frontier[i] = p
}

func (b *Braid) Add(o *orphan) {
	// 0) Dupe? Then discard.
	// 1) Link up parents of o, if we have them
	//   TODO: what if we have a parent but it is an orphan? how to get its orphan struct?
	// 2) Populate o.transitiveMissingParents
	// 3) Is o.m a missing parent? Then update other orphans and their children.
	// 4) Is o.m missing parents? Then add to b.missingParents.
	// 5) If o.m is not missing parents, and is not a missing parent, then add
	//    to b.frontier and recompute m.relContrib for all Messages on b.frontier.

	// Link up parents if we have them
	for i, pref := range o.parentRefs {
		if pmsg, ok := b.byRef.GetOk(pref); ok {
			o.parents[i].msg = pmsg
			b.children.Set(pmsg.self, append(b.children.Get(pmsg.self), o.Message))
		}
	}

	// Is o a parent of any orphans?
	for _, eo := range b.missingParents.Get(o.self) {
		tset := make(map[*orphan]struct{})
		for i, pref := range eo.parentRefs {
			if stdbytes.Equal(pref, o.self) {
				eo.parents[i].msg = o.Message
				b.children.Set(o.self, append(b.children.Get(o.self), eo.Message))

				// TODO: update transitive missing parents, check for completions
			}
		}
	}
	b.missingParents.Delete(o.self)

	// Is m an orphan? Then pull up transitive missing parents.
	for i, p := range m.parents {
		var haveParent bool
		if m.parents[i].msg, haveParent = b.byRef.GetOk(p.ref); haveParent {
			m.parents[i].ref = nil
		} else {
			pm := orphan{
				arrival: time.Now(),
				m:       m,
			}
			b.missingParents.Set(p.ref, append(b.missingParents.Get(p.ref), pm))
		}
	}

	// TODO: remove any messages on the frontier who are parents of m

	b.byRef.Set(m.self, m)
}
