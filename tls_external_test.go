package braid_test

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"net"
	"sync"
	"testing"

	"github.com/vsekhar/braid"
)

func connect(t *testing.T, h1, h2 *braid.Transport) (c1, c2 net.Conn) {
	l, err := h1.Listen("tcp", ":0")
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
		c2, err = h2.Dial(l.Addr().Network(), l.Addr().String())
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

func sendAndCheckData(t *testing.T, c1, c2 net.Conn) {
	n := 32
	buf, rbuf := make([]byte, n), make([]byte, n)
	rand.Read(buf)
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
	if !bytes.Equal(buf, rbuf) {
		t.Errorf("expected '%x', got '%x'", buf, rbuf)
	}
}

func TestSelfConnection(t *testing.T) {
	s := braid.NewIdentity()
	transport := braid.NewTransport(s)
	c1, c2 := connect(t, transport, transport)
	defer c1.Close()
	defer c2.Close()
	sendAndCheckData(t, c1, c2)
}

func TestDifferentConnect(t *testing.T) {
	transport1 := braid.NewTransport(braid.NewIdentity())
	transport2 := braid.NewTransport(braid.NewIdentity())
	c1, c2 := connect(t, transport1, transport2)
	defer c1.Close()
	defer c2.Close()
	sendAndCheckData(t, c1, c2)
}

func TestSelfIdentity(t *testing.T) {
	transport := braid.NewTransport(braid.NewIdentity())
	c1, c2 := connect(t, transport, transport)
	i1, err := braid.RemoteIdentity(c1)
	if err != nil {
		t.Fatal(err)
	}
	i2, err := braid.RemoteIdentity(c2)
	if err != nil {
		t.Fatal(err)
	}
	if !i1.Equals(i2) {
		t.Errorf("differing identities: %+v and %+v", i1, i2)
	}
}

func TestDifferentIdentity(t *testing.T) {
	i1 := braid.NewIdentity()
	transport1 := braid.NewTransport(i1)
	i2 := braid.NewIdentity()
	transport2 := braid.NewTransport(i2)
	c1, c2 := connect(t, transport1, transport2)
	ri1, err := braid.RemoteIdentity(c1)
	if err != nil {
		t.Fatal(err)
	}
	if !ri1.Equals(i2.Identity()) {
		t.Error("expected matching identities, got different identities")
	}
	ri2, err := braid.RemoteIdentity(c2)
	if err != nil {
		t.Fatal(err)
	}
	if !ri2.Equals(i1.Identity()) {
		t.Error("expected matching identities, got different identities")
	}
}
