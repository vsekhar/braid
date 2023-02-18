package locked

import "sync"

type Value[E any] struct {
	v  E
	mu sync.Mutex
}

// Get returns the current value of v. Calls to Get and Update are serialized.
func (v *Value[E]) Get() E {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.v
}

// Update passes the current value of v to f and sets v to the value returned by
// f. Calls to Get and Update are serialized.
func (v *Value[E]) Update(f func(v E) E) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.v = f(v.v)
}
