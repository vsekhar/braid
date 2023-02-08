package main

import (
	"expvar"
	"log"
	"net"

	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/vsekhar/braid/internal/framestream"
	pb "github.com/vsekhar/braid/pkg/api/braidpb"
)

var openSockets = expvar.NewInt("openSockets")

func loggedClose(c net.Conn) {
	if err := c.Close(); err != nil {
		log.Printf("(%s) error closing connection: %s", c.RemoteAddr(), err)
	}
}

func wsHandler(conn *websocket.Conn) {
	openSockets.Add(1)
	defer openSockets.Add(-1)

	// Read pump
	go func() {
		defer loggedClose(conn)

		for {
			f, err := framestream.GetPb(conn)
			if err != nil {
				log.Printf("(%s) framestream error: %s", conn.RemoteAddr(), err)
				return
			}

			switch x := f.GetPayload().(type) {
			case *pb.Frame_Noop:
				_ = x
			case *pb.Frame_RequestMessages:
			case *pb.Frame_Message:
			case *pb.Frame_RequestPeers:
			case *pb.Frame_Peer:
			default:
				log.Printf("(%s) unrecognized frame payload: %s", conn.RemoteAddr(), f.Payload)
				return
			}
		}
	}()

	var (
		reqMessages = make(chan *pb.RequestMessages)
		messages    = make(chan *pb.Message)
		reqPeers    = make(chan *pb.RequestPeers)
		peers       = make(chan *pb.Peer)
	)

	// Write pump
	go func() {
		defer loggedClose(conn)
		for {
			var f *pb.Frame

			select {
			case rmsg := <-reqMessages:
				f = &pb.Frame{Payload: &pb.Frame_RequestMessages{RequestMessages: rmsg}}
			case msg := <-messages:
				f = &pb.Frame{Payload: &pb.Frame_Message{Message: msg}}
			case rpeers := <-reqPeers:
				f = &pb.Frame{Payload: &pb.Frame_RequestPeers{RequestPeers: rpeers}}
			case peers := <-peers:
				f = &pb.Frame{Payload: &pb.Frame_Peer{Peer: peers}}
			}

			b, err := proto.Marshal(f)
			if err != nil {
				log.Printf("bad message: %s", f)
				continue
			}
			if _, err = conn.Write(b); err != nil {
				log.Printf("(%s) write error: %s", conn.RemoteAddr(), err)
				return
			}
		}
	}()
}
