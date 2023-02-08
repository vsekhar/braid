package bytes

import (
	"hash"
	"hash/fnv"
	"sync"

	"golang.org/x/exp/maps"
)

type Map[E any] struct {
	m    map[uint64]E
	f    hash.Hash64
	once sync.Once
}

func (m *Map[E]) key(b []byte) uint64 {
	m.once.Do(func() {
		m.m = make(map[uint64]E)
		m.f = fnv.New64a()
	})
	m.f.Reset()
	m.f.Write(b)
	return m.f.Sum64()
}

func (m *Map[E]) Get(b []byte) E           { return m.m[m.key(b)] }
func (m *Map[E]) GetOk(b []byte) (E, bool) { r, ok := m.m[m.key(b)]; return r, ok }
func (m *Map[E]) Has(b []byte) bool        { _, ok := m.m[m.key(b)]; return ok }
func (m *Map[E]) Set(b []byte, e E)        { m.m[m.key(b)] = e }
func (m *Map[E]) Update(m2 *Map[E])        { maps.Copy(m.m, m2.m) }
func (m *Map[E]) Delete(b []byte)          { delete(m.m, m.key(b)) }
func (m *Map[E]) Len() int                 { return len(m.m) }

func (m *Map[E]) ForEachReplace(f func(E) E) {
	for k, e := range m.m {
		m.m[k] = f(e)
	}
}

// Set is a container for unique byte slices.
type Set struct {
	m Map[struct{}]
}

// Add adds the slice b to the Set. The slice b is added based on its value at
// the time Add is called. Subsequent changes to b do not affect the Set. If b
// is already present in the Set, Add is a no-op.
func (s *Set) Add(b []byte) { s.m.Set(b, struct{}{}) }

// Has returns true if b is in the Set and false otherwise.
func (s *Set) Has(b []byte) bool { _, ok := s.m.GetOk(b); return ok }

// Delete removes b from the Set. If b is not present in the Set, Delete is a
// no-op.
func (s *Set) Delete(b []byte) { s.m.Delete(b) }

// Len returns the number of elements in the Set.
func (s *Set) Len() int { return s.m.Len() }

// Update ensures that all values in s2 are also in s1.
func (s *Set) Update(s2 *Set) { s.m.Update(&s2.m) }
