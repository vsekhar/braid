---
priority: p2
type: feature
created: 2026-03-22T22:06:43-04:00
updated: 2026-03-22T22:12:11-04:00
---

# Add -listen flag to swarm command

## Objective

Add a `-listen` flag to the swarm command so the first node can listen on a user-specified address. This enables manually connecting additional nodes (via the `braid` command) to an existing swarm.

## Context

Currently, the swarm command (`cmd/swarm/main.go`) starts all nodes on `localhost:0` (OS-assigned random ports). The `braid` command already supports a `-listen` flag (default `:8443`) for specifying a listen address. Adding the same flag to the swarm command allows a user to start a swarm with a known address for the first node, then connect external `braid` nodes to it via `-peer`.

## Location

- `cmd/swarm/main.go` — the only file that needs modification

## Approach

1. Add a `-listen` flag (default `"localhost:0"` to preserve current behavior):
   ```go
   listenAddr := flag.String("listen", "localhost:0", "listen address for the first node")
   ```

2. Use `*listenAddr` for the first node's `ListenAddr` in the node creation loop (line 37), and `"localhost:0"` for all other nodes:
   ```go
   addr := "localhost:0"
   if i == 0 {
       addr = *listenAddr
   }
   node, err := braid.NewNode(braid.NodeConfig{
       ListenAddr: addr,
       Identity:   id,
   })
   ```

3. If `NewNode` fails for the first node (e.g. address already in use), exit immediately via the existing `fatal()` helper — this already happens since the loop calls `fatal` on error. No change needed for error handling.

4. Log the first node's actual listening address after creation so the user knows where to connect:
   ```go
   slog.Info("created", "node", node.ID(), "addr", node.Addr())
   ```

## Acceptance Criteria

- [ ] `swarm -listen localhost:9000` starts the first node on `localhost:9000`; remaining nodes use random ports
- [ ] `swarm` (no flag) behaves identically to current behavior (all nodes on random ports)
- [ ] If the specified address is unavailable, the swarm exits with an error
- [ ] A `braid -peer localhost:9000` node can connect to and join the swarm

---

_📝 Noted on 2026-03-22 22:12:11-04:00 @ git:3326e77+local_

Implemented -listen flag. Used *listenAddr for node 0 and "localhost:0" for all others. Also added addr to the created log line for all nodes so the user can see where to connect. Build, vet, and tests pass. No new dead code.
