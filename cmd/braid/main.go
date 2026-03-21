package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/vsekhar/braid"
)

func main() {
	listenAddr := flag.String("listen", ":8443", "listen address")
	keyPath := flag.String("key", "braid.pem", "path to ed25519 private key")
	generateKey := flag.Bool("generate-key", false, "generate a new key and exit")
	var bootstrapPeers multiFlag
	flag.Var(&bootstrapPeers, "peer", "bootstrap peer address (repeatable)")
	flag.Parse()

	if *generateKey {
		id, err := braid.GenerateIdentity()
		if err != nil {
			fatal("generating key: %v", err)
		}
		if err := id.Save(*keyPath); err != nil {
			fatal("saving key: %v", err)
		}
		fmt.Printf("key written to %s\n", *keyPath)
		return
	}

	id, err := braid.LoadIdentity(*keyPath)
	if err != nil {
		fatal("loading key: %v\nhint: run with --generate-key to create one", err)
	}

	node, err := braid.NewNode(braid.NodeConfig{
		ListenAddr:     *listenAddr,
		Identity:       id,
		BootstrapPeers: bootstrapPeers,
	})
	if err != nil {
		fatal("creating node: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting", "node", node.ID(), "addr", node.Addr())
	if err := node.Run(ctx); err != nil && ctx.Err() == nil {
		fatal("node error: %v", err)
	}
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
