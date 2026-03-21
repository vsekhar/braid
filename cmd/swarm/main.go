package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/vsekhar/braid"
)

func main() {
	n := flag.Int("n", 10, "number of nodes")
	flag.Parse()

	if *n < 1 {
		fmt.Fprintln(os.Stderr, "need at least 1 node")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create nodes, each with an ephemeral identity and listening on a random port.
	nodes := make([]*braid.Node, *n)
	for i := range *n {
		id, err := braid.GenerateIdentity()
		if err != nil {
			fatal("generating identity for node %d: %v", i, err)
		}
		node, err := braid.NewNode(braid.NodeConfig{
			ListenAddr: "localhost:0",
			Identity:   id,
		})
		if err != nil {
			fatal("creating node %d: %v", i, err)
		}
		nodes[i] = node
		slog.Info("created", "node", node.ID())
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

	slog.Info("swarm running", "nodes", *n)
	<-ctx.Done()
	wg.Wait()
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
