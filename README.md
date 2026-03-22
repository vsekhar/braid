# Braid

Braid is a high speed distributed consensus protocol.

## Benefits

- **No mining.** Proof of work is computing a valid parent table — useful
  work that strengthens the DAG structure rather than burning cycles on
  arbitrary puzzles.
- **Natural spam resistance.** Connection admission is based on braid
  reputation. Nodes that contribute valid messages build reputation; unknown
  or misbehaving nodes are dropped before they can send data.
- **Eventually consistent.** Nodes converge on the same DAG without
  requiring synchronous rounds or leader election. Messages propagate via
  gossip and bulk sync.
- **Algorithm-agile cryptography.** Hash schemes, signature algorithms, and
  key types are versioned in the protocol buffers. Old schemes can be
  deprecated and new ones added without breaking existing messages.
- **Simple wire protocol.** Length-prefixed protobuf envelopes over mTLS.
  No custom framing, no gRPC dependency.

## Running a node

Generate a key and start a node:

```sh
go run ./cmd/braid --generate-key
go run ./cmd/braid --listen :8443 --peer host:port
```

## Running a local swarm

Start a swarm of ephemeral nodes connected in a line:

```sh
go run ./cmd/swarm -n 10
```

## License

MIT License.
