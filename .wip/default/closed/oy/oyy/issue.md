---
priority: p2
type: feature
created: 2026-03-22T22:10:36-04:00
updated: 2026-03-22T22:18:10-04:00
---

# Add -peer flag to swarm command

## Objective

Add a `-peer` flag (repeatable) to the swarm command so that swarm nodes can connect outward to external peers. This is the inverse of `wip/k4n` (which lets external nodes connect *into* a swarm via `-listen`).

## Context

The `braid` command already supports `-peer` flags for bootstrap peer connections. Adding the same to the swarm command enables two-way connectivity between swarms and standalone braid nodes. After the swarm's internal line topology is established, each `-peer` address is assigned to a randomly selected swarm node which attempts to connect to it.

Related: `wip/k4n` — Add `-listen` flag to swarm command (inbound connectivity)

## Location

- `cmd/swarm/main.go` — the only file that needs modification

## Approach

1. Add the `multiFlag` type (already exists in `cmd/braid/main.go`, lines 65-69 — duplicate it since it's 3 lines):
   ```go
   type multiFlag []string
   func (f *multiFlag) String() string { return fmt.Sprint(*f) }
   func (f *multiFlag) Set(value string) error {
       *f = append(*f, value)
       return nil
   }
   ```

2. Register the flag:
   ```go
   var peers multiFlag
   flag.Var(&peers, "peer", "external peer address to connect to (repeatable)")
   ```

3. After the internal line-topology connection loop (after line 63), connect to external peers. For each peer, randomly select a swarm node and connect it:
   ```go
   for _, peerAddr := range peers {
       node := nodes[rand.IntN(len(nodes))]
       if err := node.Connect(ctx, peerAddr); err != nil {
           slog.Error("peer connect failed", "from", node.ID(), "to", peerAddr, "err", err)
       }
   }
   ```

4. Add `"math/rand/v2"` to imports.

Note: Connection failures to external peers should be logged but not fatal — the swarm should continue running even if an external peer is unreachable. Gossip will propagate the peer address to other nodes which may retry later.

## Acceptance Criteria

- [ ] `swarm -peer localhost:8443` starts the swarm and connects a random node to `localhost:8443`
- [ ] `swarm -peer host1:8443 -peer host2:8443` connects to multiple external peers (possibly from different swarm nodes)
- [ ] `swarm` (no `-peer` flag) behaves identically to current behavior
- [ ] Failed external peer connections are logged but do not stop the swarm

---

_📝 Noted on 2026-03-22 22:18:10-04:00 @ git:b9244a7+local_

Implemented -peer flag. Added multiFlag type, registered the flag, and added loop after internal topology to connect random swarm nodes to each external peer. Failures are logged but non-fatal. Build, vet, and tests pass.
