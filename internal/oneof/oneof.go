package oneof

const (
	Unset = iota
	First
	Second
)

type Either[A, B any] struct {
	a     A
	b     B
	state uint32
}

// Zero out possibly-pointers to allow objects to be GC'd.
func (n *Either[A, B]) zeroA() A { var a A; return a }
func (n *Either[A, B]) zeroB() B { var b B; return b }

func (n *Either[A, B]) Set1(na A) { n.a = na; n.b = n.zeroB(); n.state = First }
func (n *Either[A, B]) Set2(nb B) { n.a = n.zeroA(); n.b = nb; n.state = Second }

func (n *Either[A, B]) Which() int { return int(n.state) }

func (n *Either[A, B]) Get1() A {
	if n.state != First {
		panic("oneof: type mismatch")
	}
	return n.a
}

func (n *Either[A, B]) Get2() B {
	if n.state != Second {
		panic("oneof: type mismatch")
	}
	return n.b
}
