# braid
Braid is a protocol and data structure for distributed consensus

## Commands

* `cmd/braid`: Braid is a CLI for basic signing operations
* `cmd/river`: River is a server that serves timestamps; river servers form a
braid with other river servers
* `cmd/repeater`: Repeater is a server that helps river servers scale; repeaters
are pointed to other repeaters or to a river server

> TODO: Do you need repeaters? Can you just have lots of river servers entangled
with each other?
