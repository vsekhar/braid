---
priority: p1
type: task
created: 2026-03-24T19:38:12-04:00
updated: 2026-03-24T19:41:12-04:00
---

# Add per-peer buffered send channel with writeLoop

## Objective

Add a per-peer buffered send channel with a dedicated write goroutine, so that no goroutine ever blocks on a TCP write except the writeLoop. This prevents write deadlocks where two peers block trying to write to each other, which currently causes nodes to stop receiving messages.

## Context

Analysis of long swarm runs (swarm5.log) revealed that nodes can stop receiving messages entirely. The root cause: `readLoop` calls `handleMessage` which (with reactive resolution from `wip/ibp`) calls `p.Send()` — a blocking TCP write — in the same goroutine. If the remote peer's inbound buffer is full (because it's also trying to write back), both sides deadlock: each is blocked in `Send()` inside their `readLoop`, so neither reads, so neither's write completes.

Current callers of `p.Send()`:
- `readLoop` → `handleMessage` → reactive resolution (writes resolve request back to sender)
- `readLoop` → `handleMessageRequest` → resolve response (writes resolved messages)
- `pushMessage` (from `messageLoop` timer)
- `pushGossip` (from `gossipLoop` timer)
- `sendMessageRequest` (from `wantedLoop` timer)

All of these can block on TCP writes.

## Design

### Peer changes (`peer.go`)

Add a buffered send channel and replace `Send` with non-blocking `Enqueue`:

```go
type Peer struct {
    Key         *PublicKey
    Conn        net.Conn
    ConnectedAt time.Time
    sendCh      chan *Envelope
}

// Enqueue adds an envelope to the send queue. Returns false if the queue
// is full (caller should log/drop).
func (p *Peer) Enqueue(env *Envelope) bool {
    select {
    case p.sendCh <- env:
        return true
    default:
        return false
    }
}
```

### New writeLoop (`node.go`)

A dedicated goroutine per peer that drains `sendCh` and writes to the connection:

```go
func (n *Node) writeLoop(p *Peer) {
    for env := range p.sendCh {
        if err := WriteEnvelope(p.Conn, env); err != nil {
            p.Conn.Close() // unblocks readLoop
            // drain remaining messages
            for range p.sendCh {}
            return
        }
    }
}
```

### addPeer changes (`node.go`)

Initialize `sendCh` and start `writeLoop` alongside `readLoop`:

```go
func (n *Node) addPeer(ctx context.Context, conn net.Conn, listenAddr string) {
    // ... existing setup ...
    p := &Peer{
        Key:         peerKey,
        Conn:        conn,
        ConnectedAt: time.Now(),
        sendCh:      make(chan *Envelope, 256),
    }
    // ... existing Add/log ...
    n.wg.Go(func() { n.readLoop(ctx, p) })
    n.wg.Go(func() { n.writeLoop(p) })
}
```

### readLoop changes (`node.go`)

When `readLoop` exits (read error or context cancellation), close `sendCh` to signal `writeLoop` to drain and exit:

```go
func (n *Node) readLoop(ctx context.Context, p *Peer) {
    stop := context.AfterFunc(ctx, func() { p.Conn.Close() })
    defer stop()
    defer func() {
        n.peers.Remove(p.Key)
        p.Conn.Close()
        close(p.sendCh)  // signal writeLoop to exit
        n.logger.Info("disconnected from peer", ...)
    }()
    // ... existing read loop ...
}
```

### Replace all `p.Send(env)` calls with `p.Enqueue(env)`

Callers throughout `node.go`:
- `handleMessage` (reactive resolution) — drop is fine, wantedLoop retries
- `handleMessageRequest` (resolve response) — drop is fine, requester retries
- `pushMessage` — drop is fine, message propagates via other paths
- `pushGossip` — drop is fine, gossip is periodic
- `sendMessageRequest` — drop is fine, wantedLoop retries next tick

Remove the old `Send` method from `Peer` or keep it as an internal helper used only by `writeLoop`.

### Buffer size

Start with 256. This accommodates bursts (e.g. resolve responses of many messages) while bounding memory. Can be tuned later based on analysis.

## Acceptance Criteria

- [ ] Each Peer has a `sendCh chan *Envelope` initialized in `addPeer`
- [ ] A `writeLoop` goroutine per peer drains `sendCh` and writes to conn
- [ ] All callers use `Enqueue` (non-blocking) instead of `Send` (blocking)
- [ ] `readLoop` closes `sendCh` on exit to signal `writeLoop`
- [ ] Write errors in `writeLoop` close the conn (unblocking `readLoop`)
- [ ] Existing tests pass
- [ ] Long swarm runs no longer exhibit write deadlocks

---

_📝 Noted on 2026-03-24 19:41:12-04:00 @ git:d4c2743+local_

Implemented per-peer buffered send channel with writeLoop. Added sendCh (buffered 256) to Peer struct, Enqueue() non-blocking method, writeLoop goroutine per peer. Replaced all 5 p.Send() call sites in node.go with p.Enqueue(). readLoop closes sendCh on exit to signal writeLoop. writeLoop closes conn on write error to unblock readLoop. All tests pass, no new dead code.
