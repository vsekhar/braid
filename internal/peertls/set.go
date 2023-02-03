package peertls

import (
	"hash"
	"hash/fnv"
)

type Set[T any] struct {
	m map[uint64]T
	h hash.Hash64
}

func (s *Set[T]) hash(i Identity) uint64 {
	s.h.Reset()
	s.h.Write(i.(*identity).pub)
	return s.h.Sum64()
}

func (s *Set[T]) Get(i Identity) T {
	if t, ok := s.m[s.hash(i)]; ok {
		return t
	}
	var zero T
	return zero
}

func (s *Set[T]) GetOk(i Identity) (T, bool) { t, ok := s.m[s.hash(i)]; return t, ok }
func (s *Set[T]) Set(i Identity, t T)        { s.m[s.hash(i)] = t }
func (s *Set[T]) Delete(i Identity)          { delete(s.m, s.hash(i)) }
func (s *Set[T]) Has(i Identity) bool        { _, ok := s.m[s.hash(i)]; return ok }
func (s *Set[T]) Clear()                     { s.m = make(map[uint64]T) }
func (s *Set[T]) ForEach(f func(T) error) error {
	for _, t := range s.m {
		if err := f(t); err != nil {
			return err
		}
	}
	return nil
}

func NewSet[T any]() *Set[T] {
	return &Set[T]{
		m: make(map[uint64]T),
		h: fnv.New64a(),
	}
}
