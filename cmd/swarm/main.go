package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/vsekhar/braid"
	_ "github.com/vsekhar/braid/log"
)

func main() {
	n := flag.Int("n", 10, "number of nodes")
	listenAddr := flag.String("listen", ":0", "listen address for the first node")
	var peers multiFlag
	flag.Var(&peers, "peer", "external peer address to connect to (repeatable)")
	flag.Parse()

	if *n < 1 {
		fmt.Fprintln(os.Stderr, "need at least 1 node")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down")
		cancel()
	}()

	// Create nodes, each with an ephemeral identity and listening on a random port.
	nodes := make([]*braid.Node, *n)
	for i := range *n {
		id, err := braid.GenerateIdentity()
		if err != nil {
			fatal("generating identity for node %d: %v", i, err)
		}
		addr := ":0"
		if i == 0 {
			addr = *listenAddr
		}
		node, err := braid.NewNode(braid.NodeConfig{
			ListenAddr: addr,
			Identity:   id,
		})
		if err != nil {
			fatal("creating node %d: %v", i, err)
		}
		nodes[i] = node
		slog.Info("created", "node", node.ID(), "addr", node.Addr())
	}

	// Start all nodes.
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Go(func() {
			if err := node.Run(ctx); err != nil && ctx.Err() == nil {
				slog.Error("node exited with error", "node", node.ID(), "err", err)
			}
		})
	}

	// Connect nodes in a line: 0->1->2->...->n-1.
	for i := 1; i < len(nodes); i++ {
		prevAddr := nodes[i-1].Addr().String()
		if err := nodes[i].Connect(ctx, prevAddr); err != nil {
			slog.Error("connect failed", "from", nodes[i].ID(), "to", nodes[i-1].ID(), "err", err)
		}
	}

	// Connect to external peers, each via a randomly selected swarm node.
	for _, peerAddr := range peers {
		node := nodes[rand.IntN(len(nodes))]
		if err := node.Connect(ctx, peerAddr); err != nil {
			slog.Error("peer connect failed", "from", node.ID(), "to", peerAddr, "err", err)
		}
	}

	slog.Info("swarm running", "nodes", *n)
	<-ctx.Done()
	wg.Wait()
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// multiFlag allows a flag to be specified multiple times.
type multiFlag []string

func (f *multiFlag) String() string { return fmt.Sprint(*f) }
func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
