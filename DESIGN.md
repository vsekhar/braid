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

Verification exploits the fact that all parents, their ordering, and their
claimed counts are known in advance. This allows a simultaneous multi-walk
approach that is substantially cheaper than construction.

#### Construction vs. Verification

During **construction**, the algorithm must:
- Evaluate every remaining candidate at each round to find the best.
- Discard non-winning candidates' incremental walk states each round.
- Process parents one at a time because the full set is not known in advance.
- Total: O(k × |frontier|) evaluations, each requiring an interleaved BFS.

During **verification**, the algorithm:
- Knows all k parents and their claimed counts upfront.
- Evaluates exactly 1 parent per round (or all simultaneously).
- Can reuse walk state across all parents in a single pass.
- Total: O(k) evaluations, or O(1) pass with simultaneous walks.

#### Trunk/Branch Model

In a typical braid, the parent table is heavily skewed: the first parent P_1
(the "trunk") has a count of ~total(P_1) ≈ billions, while subsequent parents
(the "branches") have counts in the 10s or 100s.

P_1's count is verified in O(1): check C_1 == total(P_1). No walk needed.
P_1's walk serves only to help branch walks terminate (by filling a visited
set that branch walks prune against).

#### Simultaneous Multi-Walk Verification

Each parent P_i is given its own walk state, seeded with P_i. All walks
proceed simultaneously with pace control. Parent indices define priority:
P_1 is highest priority, P_k is lowest.

**Backward walk step** (dequeue message M from P_i's queue):

1. Check if any higher-priority parent P_j (j < i) has already visited M.
   If so, skip — M is rightfully attributed to P_j.

2. Otherwise, add M to P_i.visited. Increment counter[i]. Enqueue M's
   parents into P_i's queue.

3. Check if any lower-priority parent P_k (k > i) has visited M. If so,
   remove M from P_k.visited and decrement counter[k] — P_i has reclaimed
   the message.

**Pace control**: expand the parent whose visited set is smallest, breaking
ties by priority (lower index first). This keeps all walks balanced and
prevents any single walk from materializing the full DAG.

**Termination**: branch walks (P_2, ..., P_k) terminate when their queues
are empty — all branches have been pruned by higher-priority visited sets
or exhausted. P_1's walk does NOT need to exhaust. Once all branch queues
are empty, P_1's remaining count is known: C_1 = total(P_1), verified O(1).

#### Forward Verification (all branch parents)

After all branch queues are empty, branch visited sets may contain **false
positives**: trunk messages that P_1's walk hadn't reached when a branch
walk claimed them. Queue exhaustion does NOT prevent this — a branch walk
can exhaust at genesis having walked through trunk territory that P_1
never visited.

Forward verification is performed for **all** branch parents simultaneously:

1. Batch BFS forward from all messages in P_2.visited ∪ ... ∪ P_k.visited
   through the children index. Each message is tagged with the parent index
   it belongs to.

2. If a forward walk from a message in P_i.visited reaches any P_j.visited
   where j < i, the message is a false positive — it is an ancestor of a
   higher-priority parent's visited message. Remove it from P_i.visited and
   decrement counter[i].

3. This is correct even if P_j.visited itself contains false positives: the
   message reaching P_j.visited proves it is in reachable(P_j), so it should
   not be attributed to P_i (which has lower priority).

After forward verification, verify counter[i] == C_i for each branch parent.

#### Cost Analysis (Verification)

- P_1 (trunk): O(1) count verification. Backward walk cost is proportional
  to the branch walks (pace control keeps them balanced), not to the trunk
  size.

- P_2, ..., P_k (branches): backward walk cost proportional to each branch's
  divergent region (typically 10s-100s of messages). Forward verification
  cost also proportional to branch sizes (forward walks hit the trunk or
  higher-priority visited sets quickly).

- **Simultaneity advantage**: all branch walks share a single forward
  verification pass. All backward walks proceed in parallel, sharing the
  benefit of P_1's growing visited set.

- **Reuse advantage**: unlike construction (which discards non-winning
  incrementals each round), verification retains all walk states and
  processes all parents in a single pass.

- **Total work**: O(sum of branch sizes) + O(P_1 walk to support pruning),
  where P_1's walk is bounded by the pace control to roughly match the
  branch sizes. This is dramatically cheaper than construction for skewed
  parent tables.
