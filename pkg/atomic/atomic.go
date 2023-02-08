package atomic

import stdatomic "sync/atomic"

type Value[E any] struct {
	val stdatomic.Value
}

func (v *Value[E]) CompareAndSwap(old, new E) (swapped bool) { return v.val.CompareAndSwap(old, new) }
func (v *Value[E]) Load() (val E)                            { return v.val.Load().(E) }
func (v *Value[E]) Store(val E)                              { v.val.Store(val) }
func (v *Value[E]) Swap(new E) (old E)                       { return v.val.Swap(new).(E) }
