---
priority: p2
type: bug
created: 2026-03-22T20:08:22-04:00
updated: 2026-03-22T20:14:29-04:00
---

# Clean up dead peers on send failure

## Objective

When a `Peer.Send` call fails (e.g., "use of closed network connection"), close the connection and remove the peer from the peer set so dead peers don't linger.

## Context

When a remote node fails, its connections close. The local node's `readLoop` already handles this on the read side — when `ReadEnvelope` returns an error, the deferred cleanup calls `peers.Remove` and `Conn.Close`. However, the send side (4 call sites in `node.go`) only logs the error and continues, leaving the dead peer in the peer set. Subsequent send attempts to that peer will also fail, wasting cycles and producing error logs until the connection maintenance loop eventually drops it.

## Location

- `node.go` — 4 `Send` call sites: `handleMessageRequest` (line ~242), `sendMessageRequest` (line ~282), `pushGossip` (line ~349), `pushMessage` (line ~378)

## Approach

On each `Send` error, call `p.Conn.Close()` after logging the error. This will cause the peer's `readLoop` to exit (since `ReadEnvelope` will fail on the closed connection), and the readLoop's deferred cleanup already handles `peers.Remove(p.Key)` and `Conn.Close()`. So closing the connection is sufficient — the readLoop defer does the rest.

`Conn.Close()` is safe to call multiple times (net.Conn documents this), so there's no double-close concern between the send error path and the readLoop defer.

At each of the 4 send error sites in `node.go`, use `errors.Is(err, net.ErrClosed)` to distinguish expected closed-connection errors from unexpected errors. Log closed connections at Info level; log other errors at Error level. In both cases, close the connection:

```go
if err := p.Send(env); err != nil {
    if errors.Is(err, net.ErrClosed) {
        n.logger.Info("connection closed from remote", "peer", publicKeyID(p.Key)[:8])
    } else {
        n.logger.Error("... send failed", "peer", publicKeyID(p.Key)[:8], "err", err)
    }
    p.Conn.Close()
    ...
}
```

For `pushMessage`, the loop over multiple peers should `continue` after closing so remaining peers still get the message.

## Acceptance Criteria

- [ ] All 4 `Send` error paths in `node.go` close the connection on failure
- [ ] Closed-connection errors (`net.ErrClosed`) are logged at Info level, not Error
- [ ] Other send errors are still logged at Error level
- [ ] Dead peers are removed from the peer set after a send failure (via readLoop cleanup)
- [ ] Existing tests continue to pass

---

_📝 Noted on 2026-03-22 20:10:37-04:00 @ git:45d55d7+local_

User clarification: on send error, use errors.Is(err, net.ErrClosed) to distinguish closed connections (log Info) from unexpected errors (log Error).

---

_📝 Noted on 2026-03-22 20:13:33-04:00 @ git:45d55d7+local_

Approach: consolidate send error handling into a single Node.sendToPeer method instead of duplicating at 4 call sites. Keeps Peer simple.

---

_📝 Noted on 2026-03-22 20:14:29-04:00 @ git:45d55d7+local_

Consolidated send error handling into Node.sendToPeer. Uses errors.Is(err, net.ErrClosed) for Info-level logging of closed connections, Error-level for other failures. Closes connection on any send error to trigger readLoop cleanup. All 4 call sites updated.
