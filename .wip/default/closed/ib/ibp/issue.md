---
priority: p2
type: task
created: 2026-03-24T16:47:31-04:00
updated: 2026-03-24T16:50:26-04:00
---

# Implement reactive resolution for pending messages

## Objective

Implement reactive resolution: when a message arrives and goes to pending (missing parents), immediately compute the missing ancestor set for that specific message and send a MessageRequest back to the peer that sent it — rather than waiting for the next 5-second `wantedLoop` tick to ask a random peer.

## Context

Analysis in `wip/gjo` and the notebook `analysis/swarm_viz.py` showed that behind nodes fall further behind because the polling-based wanted resolution (`wantedInterval=5s`, one random peer per cycle) can't keep up with the message creation rate. The sender of a message is *guaranteed* to have all ancestor messages (they must have had them to create or gossip it), making them the ideal peer to resolve missing ancestors.

## Design

### Changes to `readLoop()` (node.go:187)

Pass the peer `p` to `handleMessage()`:

```go
case *Envelope_Message:
    n.handleMessage(p, body.Message)  // was: n.handleMessage(body.Message)
```

### Changes to `handleMessage()` (node.go:214)

Accept `*Peer` parameter. After `store.Add()`, if the message went to pending, compute the wanted set for that specific message and send a targeted MessageRequest to the sender:

```go
func (n *Node) handleMessage(p *Peer, msg *Message) {
    ref, addResult, err := n.store.Add(msg)
    // ...
    if addResult.IsPending {
        // Send reactive resolution request to the sender
        env := &Envelope{
            Body: &Envelope_MessageRequest{
                MessageRequest: &MessageRequest{
                    Wanted:   addResult.MissingAncestors,
                    Frontier: n.store.Frontier(),
                },
            },
        }
        n.logger.Info("reactive resolution", "peer", publicKeyID(p.Key)[:8],
            "missing", len(addResult.MissingAncestors))
        p.Send(env)
    }
}
```

### Changes to `store.Add()` (store.go:57)

Return richer information about the outcome. Currently returns `(*MessageRef, bool, error)`. Change to return a struct:

```go
type AddResult struct {
    Ref              *MessageRef
    IsNew            bool
    IsPending        bool
    MissingAncestors []*MessageRef  // transitive missing ancestors (only if IsPending)
}
```

### New method: transitive missing ancestor walk (store.go)

When a message goes to pending, compute its missing ancestors by walking backward:

1. Start from the new message's parent refs
2. For each parent ref:
   - If **incorporated** (`s.vertices`) → stop, we have it
   - If **pending** (`s.pending`) → recurse into *its* parents (it's present but blocked, so its missing parents transitively block us too)
   - If **neither** (not in `vertices` or `pending`) → add to the wanted/missing set
3. Return the collected missing refs

This is similar to the existing `Resolve()` BFS but in the opposite direction — instead of walking backward from wanted to find messages to send, we walk backward from a pending message to find refs we need.

### Existing `wantedLoop` (node.go:255)

Keep the existing `wantedLoop` as a fallback. It handles edge cases:
- Messages received during startup before peers are fully connected
- Resolution failures (sender disconnects before responding)
- Any refs that slip through the reactive path

## Edge Cases

- **Sender disconnects before responding**: The wantedLoop fallback handles this
- **Cyclic pending chains**: The BFS should track visited refs to avoid infinite loops
- **Concurrent adds**: The walk happens inside `store.Add()` which holds `s.mu.Lock()`, so it's safe
- **Large missing sets**: Could be bounded like `maxResolve`, but initially let it be unbounded since the sender is guaranteed to have everything

## Acceptance Criteria

- [ ] `readLoop()` passes peer to `handleMessage()`
- [ ] `store.Add()` returns pending status and missing ancestor refs
- [ ] Missing ancestor walk is transitive (walks through pending messages)
- [ ] When a message goes to pending, a MessageRequest is immediately sent to the sender
- [ ] Existing `wantedLoop` continues to work as a fallback
- [ ] Existing tests pass
- [ ] Add log line for reactive resolution requests (for analysis notebook)

---

_📝 Noted on 2026-03-24 16:50:26-04:00 @ git:34e6d29+local_

Implemented reactive resolution. Changes: (1) store.go: Added AddResult struct, modified Add() to return it with IsPending/MissingAncestors, added transitiveMissing() BFS walk; (2) node.go: handleMessage() now takes *Peer param, sends immediate MessageRequest to sender when message goes to pending. All tests pass, no new dead code.
