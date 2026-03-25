---
priority: p2
type: feature
created: 2026-03-24T21:57:45-04:00
updated: 2026-03-24T22:22:34-04:00
---

# Implement have/want negotiation protocol for efficient resolve

## Objective

Replace the single-round-trip resolve protocol with a symmetric, bidirectional have/want synchronization protocol. Two connected peers continuously exchange `HaveWant` metadata messages to discover what each is missing, while actual content flows as regular `Message` protobufs on the existing data path. This eliminates the current over-sending problem where resolve batches approach the size of the entire message graph.

## Problem

The current `Resolve()` protocol uses the requester's frontier (DAG tips) as the stop set for a backward walk from wanted refs. This fails when the responder doesn't recognize some frontier refs (because the requester has messages the responder hasn't seen). Unrecognized frontier refs provide no pruning, so the walk traverses nearly the entire graph. Resolve batch sizes of 2500-3000 have been observed in a network with ~3000 total messages.

The stop set must be a **complete cut** of the DAG that **both nodes recognize** (a "shared cut set"). Finding this requires negotiation — no single-round-trip protocol can compute it.

Simpler alternatives don't work:
- **Responder's frontier as stop set**: The responder's tips are descendants of the wanted refs; walking backward moves away from them.
- **Sampling the requester's incorporated refs**: Unless the sample forms a complete cut, the walk routes around sampled refs via unsampled branches.

## Design

### Separation of concerns

- **Control plane**: `HaveWant` messages — metadata about what each node has and needs. Purely refs, no message content.
- **Data plane**: `Message` protobufs — unchanged. Nodes send and receive Messages exactly as they do now, incorporating them via `store.Add()`.

The have/want exchange makes the data plane smarter about *which* messages to send. When a node discovers from the exchange that its peer wants something it has, it sends a regular `Message`.

### New proto

```protobuf
message HaveWant {
    repeated MessageRef have = 1;  // refs the sender has (proposed cut)
    repeated MessageRef want = 2;  // refs the sender needs
}
```

Added to the `Envelope` oneof alongside `Message`, `PeerGossip`, and `MessageRequest`.

### Protocol flow

When node A wants to sync with node B:

**Round 1:** A sends `HaveWant{have: A's frontier, want: A's wanted refs}`

**B processes A's message:**
- For each ref in `have`:
  - If B recognizes it (in B's vertices) → shared cut grows. B now knows A has this and everything behind it.
  - If B does NOT recognize it → B adds it to its own `want` set. This is something A has that B doesn't.
- For each ref in `want`:
  - If B has the message → B sends it as a regular `Message` protobuf.
  - If B doesn't have it → B can't help (the ref stays in A's wanted set for other peers).

**B responds:** `HaveWant{have: B's frontier, want: B's wanted refs (including unrecognized refs from A's have)}`

**A processes B's response** using the same logic. Each exchange simultaneously:
1. **Refines the shared cut set** — recognized `have` refs establish common ground
2. **Transfers content** — `want` refs fulfilled by sending regular Messages
3. **Discovers new gaps** — unrecognized `have` refs become the receiver's `want` refs

**Steady state:** The exchange runs continuously for the lifetime of the peer connection. When a received `HaveWant` has a fully recognized `have` set and an empty or unsatisfiable `want` set, that particular message is a no-op — but the exchange continues, since both nodes' state changes as they create or learn new messages. The protocol is always running, not request-response.

### Key insight

An unrecognized `have` ref is simultaneously:
- Evidence of the sender's state (the sender has this message and its ancestors)
- A `want` for the receiver (the receiver doesn't have it)

This dual-use means every piece of information in the exchange does double duty. There is no separate "negotiation phase" followed by a "transfer phase" — they are interleaved from the first message.

### Continuous sync

The protocol is inherently continuous — `HaveWant` messages are exchanged periodically for the lifetime of each peer connection. There is no "start" or "finish" to the protocol; it simply runs. When nodes are in sync, the exchanges are no-ops. When state diverges (new messages created, new messages learned from other peers), the next exchange detects and resolves the gap. This replaces both:
- **Reactive resolution** (currently triggered per pending message in `handleMessage`)
- **wantedLoop** (currently polls every 5 seconds)

### Self-correction

As content flows (Messages sent in response to `want` refs), the receiving node incorporates them and its frontier advances. The next `have` set reflects this, naturally reducing the volume of subsequent exchanges. The protocol is self-correcting — progress on the data plane feeds back into the control plane.

## Files to modify

- `braid.proto` — add `HaveWant` to `Envelope` oneof
- `node.go` — add `handleHaveWant(p *Peer, hw *HaveWant)` in `readLoop` switch
- `node.go` — initiate have/want exchange on new peer connection (in `addPeer`)
- `node.go` — reactive resolution and `wantedLoop` may become obsolete, replaced by continuous sync
- `store.go` — may need a helper to compute the `have` set (frontier or a broader cut)
- `MessageRequest` proto and `handleMessageRequest`/`Resolve` may become obsolete

## Acceptance Criteria

- [ ] `HaveWant` proto message added to `Envelope` oneof
- [ ] Nodes exchange `HaveWant` messages to discover shared cut set
- [ ] Unrecognized `have` refs become `want` refs for the receiver
- [ ] `want` refs fulfilled by sending regular `Message` protobufs (existing data path)
- [ ] Resolve batch sizes proportional to the actual gap, not the full graph
- [ ] Exchange runs continuously per peer; no-op when in sync
- [ ] Works bidirectionally — both nodes learn simultaneously
- [ ] Backward compatible: nodes handle peers that don't support the new protocol
