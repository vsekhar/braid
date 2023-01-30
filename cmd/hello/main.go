package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
)

var port = flag.Int("port", 0, "port to listen on (defaults to an ephemeral port)")

func main() {
	flag.Parse()
	bctx := context.Background()
	ctx, cancel := signal.NotifyContext(bctx, os.Interrupt)
	defer cancel()

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on: http://localhost:%d", l.Addr().(*net.TCPAddr).Port)

	srv := http.Server{}
	go func() {
		if err := srv.Serve(l); err != nil {
			log.Print(err)
		}
	}()

	<-ctx.Done()
	if err := srv.Shutdown(bctx); err != nil {
		log.Print(err)
	}
}
