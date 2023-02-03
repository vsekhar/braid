package peertls_test

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"net"
	"sync"
	"testing"

	"github.com/vsekhar/braid/internal/peertls"
)

func selfConnect(t *testing.T, h *peertls.Host) (c1, c2 net.Conn) {
	l, err := h.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		c1, err = l.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		if err := c1.(*tls.Conn).Handshake(); err != nil {
			t.Error(err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		c2, err = h.Dial(l.Addr().Network(), l.Addr().String())
		if err != nil {
			t.Error(err)
			return
		}
		if err := c2.(*tls.Conn).Handshake(); err != nil {
			t.Error(err)
		}
	}()
	wg.Wait()
	return
}

func TestConnection(t *testing.T) {
	n := 32
	buf, rbuf := make([]byte, n), make([]byte, n)
	rand.Read(buf)

	s := peertls.NewIdentity()
	h := peertls.NewHost(s)
	c1, c2 := selfConnect(t, h)
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := c1.Write(buf); err != nil {
			t.Error(err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := c2.Read(rbuf); err != nil {
			t.Error(err)
		}
	}()
	wg.Wait()
	defer c1.Close()
	defer c2.Close()
	if !bytes.Equal(buf, rbuf) {
		t.Errorf("expected '%x', got '%x'", buf, rbuf)
	}
}

func TestIdentity(t *testing.T) {
	h := peertls.NewHost(peertls.NewIdentity())
	c1, c2 := selfConnect(t, h)
	i1, err := peertls.RemoteIdentity(c1)
	if err != nil {
		t.Fatal(err)
	}
	i2, err := peertls.RemoteIdentity(c2)
	if err != nil {
		t.Fatal(err)
	}
	if !i1.Equals(i2) {
		s1, _ := i1.DebugMarshalText()
		s2, _ := i2.DebugMarshalText()
		t.Errorf("differing identities: %s and %s", s1, s2)
	}
}
