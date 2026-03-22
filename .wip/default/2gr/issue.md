---
priority: p2
type: feature
created: 2026-03-22T19:50:53-04:00
updated: 2026-03-22T19:50:53-04:00
---

# Add periodic message creation and push gossip to peers

## Objective

Add a periodic message creation loop so each node creates a new message every 500ms, incorporates it into its local braid, and pushes it to up to 5 random connected peers.

## Context

The node currently has three periodic loops: peer gossip (7s), connection maintenance (3s), and wanted requests (5s). Messages are only synced reactively via pull-based wanted requests. This issue adds active message creation and push-based dissemination, which is the core mechanism for nodes to participate in the braid protocol.

## Location

- `node.go` — new `messageLoop` goroutine, `pushMessage` method, `messageInterval` constant, wiring in `Run`
- `peer.go` — new `RandomN(n int) []*Peer` method on `PeerSet`

## Approach

### 1. Add `RandomN` to `PeerSet` (`peer.go`)

Add a method `RandomN(n int) []*Peer` that returns up to `n` random peers (or all peers if fewer than `n` are connected). Use Fisher-Yates shuffle on the `All()` snapshot and take the first `min(n, len)` elements. This follows the existing `Random()` pattern.

### 2. Add message creation loop (`node.go`)

Add the constant:
```go
const messageInterval = 500 * time.Millisecond
```

Add a `messageLoop(ctx context.Context)` method following the same ticker pattern as `gossipLoop`, `connectLoop`, and `wantedLoop`.

Add a `pushMessage()` method that:
1. Calls `n.store.CreateMessage(n.cfg.Identity)` to create, sign, and incorporate a new message
2. Calls `n.peers.RandomN(5)` to select up to 5 random connected peers
3. Wraps the message in `&Envelope{Body: &Envelope_Message{Message: msg}}` and calls `p.Send(env)` for each selected peer
4. Logs the message creation and send (ref, number of peers sent to, store stats)

If there are no connected peers, the message is still created and incorporated locally (it will be synced later via wanted requests).

### 3. Wire up in `Run` (`node.go`)

Add a new goroutine in `Run` alongside the existing loops:
```go
// Message creation loop.
n.wg.Go(func() {
    n.messageLoop(ctx)
})
```

### Notes

- The receiving side already works: `readLoop` dispatches `Envelope_Message` to `handleMessage`, which calls `store.Add` and logs the result.
- Messages with missing parents will be buffered as pending and resolved via the existing wanted mechanism.
- No changes needed to the protobuf schema or wire format.

## Acceptance Criteria

- [ ] `const messageInterval = 500 * time.Millisecond` defined in `node.go`
- [ ] `PeerSet.RandomN(n)` returns up to n random peers from the set
- [ ] `messageLoop` creates a message every 500ms and sends it to up to 5 random peers
- [ ] Messages are incorporated locally even when no peers are connected
- [ ] Existing tests continue to pass
- [ ] The swarm simulator (`cmd/swarm/main.go`) shows messages being created and exchanged between nodes
