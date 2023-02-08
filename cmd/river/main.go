package main

import (
	"flag"
	"fmt"
	"log"

	_ "expvar"
	"net/http"
	_ "net/http/pprof"

	"golang.org/x/net/websocket"
)

var port = flag.Uint("port", 8080, "listen port (default 8080)")

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	http.Handle("/ws", websocket.Handler(wsHandler))

	hostPort := fmt.Sprintf("localhost:%d", *port)
	log.Println(http.ListenAndServe(hostPort, nil))
}
