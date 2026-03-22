# Braid: Design Document

## Parent Table Construction

### Definitions

- **reachable(X)**: the set of all messages backward-reachable from X (including X)
- **total(X)**: |reachable(X)| = sum of all message_counts in X's parent table + 1
- **unique(S)**: |union of reachable(X) for all X in S|
- **additional(B | S)**: |reachable(B) \ reachable(S)| — messages reachable from
  B that are NOT reachable from any message in set S

### Parent Table Invariants

The parent table of a message M is an ordered list of (parent_ref, message_count):

1. Entries are ordered by message_count, highest first.
2. Ties broken by timestamp (later first), then lexicographically on ref.
3. For entry i (parent P_i):
   message_count = unique({P_1, ..., P_i}) - unique({P_1, ..., P_{i-1}})
4. For the first entry: message_count = total(P_1).
5. Sum of all message_counts = unique({all parents}) = |reachable(M) \ {M}|.

### Determinism

Given a chosen set of parents, there is exactly one valid parent table. The
ordering and counts are interdependent: the count of each parent depends on
which parents precede it, and the ordering is by count. The algorithm must
greedily select the next parent at each step.

A node may choose which frontier messages to include as parents, but for any
chosen set, the resulting parent table is deterministic.

### Parent Selection Algorithm

When creating a new message, given frontier F:

**Round 1**: Compute total(X) for each X in F. This is O(1) per message
(sum of message_counts in X's parent table + 1). Select P_1 = argmax total(X)
(ties: later timestamp, then lex ref). P_1.message_count = total(P_1).

**Round 2+**: For each remaining candidate X in F, compute
additional(X | {P_1, ..., P_{i-1}}) using the interleaved backward BFS
algorithm described below. Select P_i = argmax additional. Continue until all
chosen parents have been ordered.

### Walk State

The algorithm uses a `walkState` struct consisting of:
- **visited**: set of message refKeys that have been processed (dequeued and
  their parents enqueued)
- **queue**: messages to be visited next (not yet in visited). When a message
  is dequeued, it is added to visited and its parents are added to the queue.

Two walk states can be **merged**: `A.merge(B)` unions the visited sets and
appends B's queue entries not already in A's visited set to A's queue. This
preserves A's expansion frontier while incorporating B's discoveries.

An **accumulator** walk state persists across rounds, tracking the covered
region (messages reachable from all previously selected parents). After each
round, the winning candidate's incremental walk state is merged into the
accumulator, and the selected parent is seeded into the accumulator's queue.

### Computing additional(B | covered)

This is the core computation. It uses two phases:

#### Phase 1: Interleaved Backward BFS

Two walks proceed simultaneously:
- **A side (accumulator)**: expands the accumulator's walk state in place,
  growing the covered region. Dequeues from the accumulator's queue, adds
  messages to its visited set, and enqueues their parents.
- **B side (candidate)**: walks backward from the candidate, building an
  **incremental** walk state. Messages found in the accumulator's visited set
  are pruned (not counted, parents not enqueued). Messages not in the
  accumulator's visited set are added to the incremental visited set.

**Pace control**: expand A when `|accum.visited| <= |incremental.visited|`,
expand B otherwise. This adaptive balance ensures:
- Neither set grows much larger than the divergent region.
- A's walk fills the accumulator with enough coverage for B to prune.
- B's walk doesn't materialize the full reachable set of the candidate.

**Correction**: when A's walk discovers a message already in the incremental
set, that message is removed from incremental (it was a false positive —
reachable from the covered roots but not yet discovered when B visited it).

Phase 1 ends when B's queue is empty — all of B's branches have either been
pruned by the accumulator's visited set or exhausted.

#### Phase 2: Forward Verification

After Phase 1, the incremental set may contain **false positives**: messages
that are in reachable(covered roots) but were visited by B before A's walk
reached them, and were never corrected because A's walk didn't expand far
enough in that direction.

To catch these, we do a batch BFS **forward** through the children index
from all remaining incremental messages simultaneously. If a forward walk
from an incremental message reaches any message in the accumulator's visited
set, that incremental message is an ancestor of a visited message and
therefore is in reachable(covered roots). It is removed from incremental as
a false positive.

This is efficient because:
- The incremental set is small (bounded by the divergent region).
- Forward walks in a braid go toward the frontier, which is close.
- The batch BFS shares structure across forward walks.
- We stop at the **first** visited message reached per root.

#### Cost Analysis

The total work is proportional to the **divergent region** between the
candidate and the covered roots — messages between the two that are not
in their shared history.

- **Healthy braid**: convergence is rapid, divergent region is small. Both
  phases complete quickly.
- **Degenerate case**: long-divergent branches produce a large divergent
  region. This expense is intentional — it is the proof of work.

The pace control ensures that neither A's visited set nor B's incremental
set grows much larger than the divergent region. The total memory is bounded
by approximately 2× the divergent region size.

The interleaved walk is correct regardless of the pace ratio. Walking A
faster means fewer false positives for forward verification to catch. Walking
A slower means more forward verification work. The total verification work
(A backward + forward verification) is the same either way — both explore
the same boundary between the accumulator's visited set and the shared
region, just from opposite directions. The pace control provides a natural
adaptive balance.

### Proof of Work Property

Computing the parent table is intentionally expensive. It serves as a natural
proof of work:
- A node that cannot compute a valid parent table cannot create messages.
- A stale or under-resourced node can wait for additional messages to arrive
  that merge branches, reducing the divergent region.
- Other nodes verify the counts when validating a message, requiring the same
  core computation.
- The counts must be exact. Approximate counts are rejected by other nodes.

### Verification

When verifying a received message's parent table, the same algorithm is used:
- An accumulator walk state is built up across entries.
- For each entry, `additional()` computes the expected count.
- The claimed count must match exactly.
- Ordering must be descending by count, with ties broken by timestamp (later
  first) then lexicographically on ref.

Verification is computationally equivalent to construction. The accumulator
grows across entries just as it does across rounds during construction.
