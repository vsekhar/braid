# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "marimo",
#     "matplotlib",
#     "networkx",
# ]
# ///

import marimo

__generated_with = "0.21.1"
app = marimo.App(width="full")


@app.cell
def _():
    import marimo as mo

    return (mo,)


@app.cell(hide_code=True)
def _(mo):
    mo.md(r"""
    # DAG Diff Algorithm Trace

    Simulate and visualize different algorithms for computing the diff between two nodes' DAGs.

    - Build a shared DAG, then fork it so node A and node B have overlapping but different message sets.
    - Trace the **cursor-based reconciliation** algorithm tick by tick.
    - Visualize the DAG, highlighting shared vs. divergent regions and the search state.
    """)
    return


@app.cell
def _():
    from dataclasses import dataclass, field
    import hashlib, itertools, random
    from typing import Optional

    @dataclass
    class Msg:
        """A message in the DAG."""
        id: str
        parents: list[str] = field(default_factory=list)
        generation: int = 0
        # computed
        _ancestors: Optional[set] = field(default=None, repr=False)

    class DAG:
        """Simple in-memory DAG of messages."""

        def __init__(self):
            self.msgs: dict[str, Msg] = {}  # id → Msg

        def copy(self) -> "DAG":
            d = DAG()
            for m in self.msgs.values():
                d.msgs[m.id] = Msg(
                    id=m.id,
                    parents=list(m.parents),
                    generation=m.generation,
                )
            return d

        def add(self, id: str, parents: list[str]) -> Msg:
            gen = max((self.msgs[p].generation for p in parents), default=-1) + 1
            m = Msg(id=id, parents=parents, generation=gen)
            self.msgs[m.id] = m
            return m

        def has(self, id: str) -> bool:
            return id in self.msgs

        def frontier(self) -> set[str]:
            """Messages with no children."""
            has_child = set()
            for m in self.msgs.values():
                for p in m.parents:
                    has_child.add(p)
            return {id for id in self.msgs if id not in has_child}

        def ancestors(self, id: str) -> set[str]:
            """All ancestors of id (not including id)."""
            m = self.msgs[id]
            if m._ancestors is not None:
                return m._ancestors
            result: set[str] = set()
            stack = list(m.parents)
            while stack:
                cur = stack.pop()
                if cur in result:
                    continue
                result.add(cur)
                stack.extend(self.msgs[cur].parents)
            m._ancestors = result
            return result

        def parent_walk(self, start: str, hops: int, follow: str = "all") -> set[str]:
            """Walk `hops` levels back through parents from `start`.

            follow:
              "all"    – follow all parents at each step
              "branch" – follow the highest-generation parent at each step
            Returns the set of message ids found at exactly `hops` levels back.
            """
            current = {start}
            for _ in range(hops):
                nxt: set[str] = set()
                for cid in current:
                    msg = self.msgs.get(cid)
                    if msg is None or not msg.parents:
                        continue
                    if follow == "branch":
                        best = max(msg.parents, key=lambda p: self.msgs[p].generation)
                        nxt.add(best)
                    else:
                        nxt.update(msg.parents)
                current = nxt
                if not current:
                    break
            return current

        def children_of(self, ids: set[str]) -> set[str]:
            """All messages that have a parent in `ids`."""
            result: set[str] = set()
            for m in self.msgs.values():
                if any(p in ids for p in m.parents):
                    result.add(m.id)
            return result

        def forward_walk(self, boundary: set[str]) -> list[str]:
            """BFS forward from `boundary` (inclusive), returning messages in generation order.

            `boundary` is the set of deepest miss refs. The walk includes
            the boundary refs themselves plus all their descendants.
            """
            visited = set(boundary)
            queue = list(boundary)
            result: list[str] = list(boundary)  # inclusive: boundary refs are part of the delta
            while queue:
                cid = queue.pop(0)
                for m in self.msgs.values():
                    if m.id in visited:
                        continue
                    if any(p in visited for p in m.parents):
                        visited.add(m.id)
                        queue.append(m.id)
                        result.append(m.id)
            # sort by generation for topological order
            result.sort(key=lambda x: self.msgs[x].generation)
            return result

    return (DAG,)


@app.cell
def _(DAG, mo):
    mo.md(r"""
    ## Example DAG

    Build a shared trunk, then fork: A gets some extra messages, B gets different extra messages.
    Both share the trunk and some overlap beyond it.
    """)

    def build_example() -> tuple["DAG", "DAG", dict]:
        """Build two DAGs that share a common trunk but diverge."""
        base = DAG()

        # Shared trunk: g → t1 → t2 → ... → t10
        base.add("g", [])
        for i in range(1, 11):
            base.add(f"t{i}", [f"t{i-1}" if i > 1 else "g"])

        # Shared branch from t5
        base.add("s1", ["t5"])
        base.add("s2", ["s1"])
        base.add("s3", ["s2"])

        # Shared merge at t8
        base.add("m1", ["t8", "s3"])

        # Shared continuation
        base.add("c1", ["m1"])
        base.add("c2", ["t10", "c1"])

        # Fork A: extra messages only A has
        dag_a = base.copy()
        dag_a.add("a1", ["c2"])
        dag_a.add("a2", ["a1"])
        dag_a.add("a3", ["a2"])
        # A also has a side branch
        dag_a.add("a_side1", ["t10"])
        dag_a.add("a_side2", ["a_side1"])
        dag_a.add("a4", ["a3", "a_side2"])  # merge

        # Fork B: extra messages only B has
        dag_b = base.copy()
        dag_b.add("b1", ["c2"])
        dag_b.add("b2", ["b1"])

        info = {
            "shared": set(base.msgs.keys()),
            "only_a": set(dag_a.msgs.keys()) - set(base.msgs.keys()),
            "only_b": set(dag_b.msgs.keys()) - set(base.msgs.keys()),
        }
        return dag_a, dag_b, info

    dag_a, dag_b, dag_info = build_example()

    mo.md(f"""
    **Node A**: {len(dag_a.msgs)} messages, frontier = `{dag_a.frontier()}`

    **Node B**: {len(dag_b.msgs)} messages, frontier = `{dag_b.frontier()}`

    **Shared**: {len(dag_info['shared'])} messages

    **Only A**: `{dag_info['only_a']}`

    **Only B**: `{dag_info['only_b']}`
    """)
    return dag_a, dag_b, dag_info


@app.cell
def _(dag_a, dag_b, dag_info, mo):
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
    import networkx as nx

    def draw_dag(dag, title, only_mine=None, only_theirs=None, active=None, boundary=None):
        """Draw a DAG with regions colored."""
        G = nx.DiGraph()
        for m in dag.msgs.values():
            G.add_node(m.id)
            for p in m.parents:
                G.add_edge(m.id, p)  # edge from child to parent

        # Layout: use generation for vertical position
        pos = {}
        by_gen: dict[int, list[str]] = {}
        for m in dag.msgs.values():
            by_gen.setdefault(m.generation, []).append(m.id)
        for gen, ids in by_gen.items():
            ids_sorted = sorted(ids)
            for i, mid in enumerate(ids_sorted):
                x = i - (len(ids_sorted) - 1) / 2
                pos[mid] = (x, gen)

        colors = []
        for nid in G.nodes():
            if active and nid in active:
                colors.append("#ff4444")       # red: active miss frontier
            elif boundary and nid in boundary:
                colors.append("#44cc44")       # green: confirmed boundary
            elif only_mine and nid in only_mine:
                colors.append("#4488ff")       # blue: only this node
            elif only_theirs and nid in only_theirs:
                colors.append("#cccccc")       # gray: only the other node
            else:
                colors.append("#ffffaa")       # yellow: shared

        fig, ax = plt.subplots(1, 1, figsize=(10, 8))
        nx.draw(
            G, pos, ax=ax,
            with_labels=True,
            node_color=colors,
            node_size=800,
            font_size=8,
            font_weight="bold",
            arrows=True,
            arrowsize=15,
            edge_color="#999999",
        )
        ax.set_title(title, fontsize=14)

        patches = [
            mpatches.Patch(color="#ffffaa", label="Shared"),
            mpatches.Patch(color="#4488ff", label="Only mine"),
            mpatches.Patch(color="#cccccc", label="Only theirs (unknown)"),
        ]
        if active:
            patches.append(mpatches.Patch(color="#ff4444", label="Proposals (searching)"))
        if boundary:
            patches.append(mpatches.Patch(color="#44cc44", label="Boundary (converged)"))
        ax.legend(handles=patches, loc="lower right", fontsize=8)
        return fig

    fig_a = draw_dag(dag_a, "Node A's DAG", only_mine=dag_info["only_a"], only_theirs=dag_info["only_b"])
    fig_b = draw_dag(dag_b, "Node B's DAG", only_mine=dag_info["only_b"], only_theirs=dag_info["only_a"])
    mo.vstack([fig_a, fig_b])
    return (draw_dag,)


@app.cell
def _(mo):
    mo.md(r"""
    ## Proposal-Based Reconciliation Algorithm

    Both nodes run the algorithm simultaneously. Each node maintains a set of
    **proposals** — refs it is currently testing with the peer. Proposals move
    bidirectionally through the DAG (toward frontier or toward genesis) based on
    the peer's responses, converging on the boundary between shared and unshared
    regions.

    The protocol uses three fields per message:
    - `probe`: refs we are testing — "I have these. Do you also have these?"
    - `have`: responses to peer's probes — "yes, I also have these"
    - `want`: responses to peer's probes — "no, I don't have these"

    **State (per peer):**
    ```python
    proposals: dict[str, tuple[int, int|None]]
        # ref → (last_hop, range)
        # last_hop: signed — positive = forward, negative = backward
        # range: None = expansion phase (doubling allowed)
        #        int  = narrowing phase (halving only, range shrinks each step)

    cache: dict[str, int]  # cached peer responses: ref → +1 (hit) or -1 (miss)
                           # fast-forwards proposals on already-probed refs
                           # refs with cache[ref] == +1 are confirmed shared —
                           # walks skip these to avoid re-exploring shared territory
    ```

    **Bootstrap:**
    - Add frontier tips to proposals with `last_hop = -1, range = None`.
    - Send `probe = [proposal refs]` (the frontier tips).

    **Tick (triggered by receiving a message from the peer):**

    *1. Respond to peer's probes:*
    - For each ref in peer's `probe`: check if we have it.
      - Have it → include in our outgoing `have`.
      - Don't have it → include in our outgoing `want`.

    *2. Process peer's responses to our probes:*
    - For each ref in peer's `have` or `want`:
      - If ref is in `proposals`: update per the hop rules below.
      - Else: ignore (async tolerance, or peer's own protocol traffic).

    Hop rules for proposals (ref has `last_hop` and `range`):
      - Determine response direction: hit → forward (+1), miss → backward (-1).
      - **Expansion phase** (`range` is None):
        - Same direction as `last_hop`: `next_hop = last_hop * 2` (keep going).
        - Different direction (first flip): `range = abs(last_hop)`.
          Switch to narrowing.
      - **Narrowing phase** (`range` is set):
        - `next_mag = range // 2`, direction from response. `range = next_mag`.
      - If `next_mag == 0`: **converged**.
        - Miss → boundary ref. Remove from proposals, add to boundary.
        - Hit → correction step: walk +1, children become proposals with
          `last_hop = +1, range = 1`. When those children are probed:
          miss → boundary. Hit → drop (shared ground, no boundary here).
      - Otherwise: walk `next_hop` from ref. All refs reached become new
        proposals inheriting `last_hop = next_hop` and the current `range`.
        Skip refs already in `shared` (confirmed hits — no point re-probing).
        If ALL reached refs are shared, halve and retry immediately (fast-forward
        through confirmed shared territory). If narrowed to 0 with all shared:
        miss → boundary, hit → drop.
        Remove old proposal.
    - New proposals are checked against the response cache. If a cached response
      exists, the proposal is processed immediately (same hop rules) without
      waiting for a round trip. This eliminates redundant probes for refs
      reached via multiple DAG paths.
    - Proposals with no response (and no cached response) stay unchanged (async tolerance).

    *3. Send probes:*
    - Send `probe = [proposal refs]` for proposals not resolved from cache.

    **Termination:**
    - When `proposals` is empty, all paths have converged.
    - The converged refs (removed in step 2) are the boundary.
    - Delta = forward walk from boundary, inclusive.
    """)
    return


@app.cell
def _():
    from dataclasses import dataclass as _dataclass

    @_dataclass
    class TickRecord:
        """What happened on one tick, from one node's perspective."""
        tick: int
        node: str                    # "A" or "B"
        # State at start of tick
        proposals_before: dict       # ref → (last_hop, range)
        # Received message
        received_probe: set          # peer's probes
        received_have: set           # peer's responses: has these
        received_want: set           # peer's responses: doesn't have these
        # What happened
        converged_this_tick: set      # refs that converged (boundary misses)
        correction_this_tick: set     # refs that need correction step (boundary hits)
        # State after update
        proposals_after: dict        # ref → (last_hop, range)
        boundary: set                # all converged boundary refs so far
        # Message sent
        sent_probe: set
        sent_have: set
        sent_want: set

    def _walk(dag, ref, hop):
        """Walk `hop` from ref. Positive = forward (children), negative = backward (parents).
        Returns set of refs reached at exactly |hop| distance."""
        magnitude = abs(hop)
        if magnitude == 0:
            return {ref}
        current = {ref}
        for _ in range(magnitude):
            nxt = set()
            for cid in current:
                if hop > 0:
                    # Forward: find children
                    nxt.update(dag.children_of({cid}))
                else:
                    # Backward: follow parents
                    msg = dag.msgs.get(cid)
                    if msg:
                        nxt.update(msg.parents)
            current = nxt
            if not current:
                break
        return current

    def run_duplex_reconciliation(
        dag_a: "DAG",
        dag_b: "DAG",
        max_ticks: int = 30,
    ) -> list[TickRecord]:
        """Simulate proposal-based bidirectional reconciliation between A and B."""

        # Per-node state
        proposals_a = {f: (-1, None) for f in dag_a.frontier()}
        proposals_b = {f: (-1, None) for f in dag_b.frontier()}
        boundary_a: set[str] = set()
        boundary_b: set[str] = set()
        cache_a: dict[str, int] = {}  # cached peer responses: ref → +1 or -1
        cache_b: dict[str, int] = {}
        records: list[TickRecord] = []

        # Bootstrap: probes are the frontier tips (already in proposals)
        a_probe = set(proposals_a.keys())
        a_have: set[str] = set()
        a_want: set[str] = set()
        b_probe = set(proposals_b.keys())
        b_have: set[str] = set()
        b_want: set[str] = set()

        for tick in range(max_ticks):
            for node, my_dag, peer_dag, proposals, boundary, cache, \
                recv_probe, recv_have, recv_want in [
                ("A", dag_a, dag_b, proposals_a, boundary_a, cache_a,
                 b_probe, b_have, b_want),
                ("B", dag_b, dag_a, proposals_b, boundary_b, cache_b,
                 a_probe, a_have, a_want),
            ]:
                proposals_before = dict(proposals)

                # --- 1. Respond to peer's probes ---
                out_have = set()
                out_want = set()
                for ref in recv_probe:
                    if my_dag.has(ref):
                        out_have.add(ref)
                    else:
                        out_want.add(ref)

                # --- 2. Process peer's responses to our probes ---
                converged = set()
                correction = set()

                # Cache incoming responses
                for ref in recv_have:
                    cache[ref] = 1   # hit
                for ref in recv_want:
                    cache[ref] = -1  # miss

                def _is_shared(ref):
                    return cache.get(ref) == 1

                def _process_proposal(ref, last_hop, rng, new_proposals):
                    """Process a single proposal against cache. May recurse
                    for new proposals that have cached responses."""
                    if ref not in cache:
                        # No response — carry forward unchanged
                        new_proposals[ref] = (last_hop, rng)
                        return

                    response_dir = cache[ref]
                    last_sign = 1 if last_hop > 0 else -1
                    last_mag = abs(last_hop)

                    if rng is None:
                        if last_sign == response_dir:
                            next_mag = last_mag * 2
                            next_rng = None
                        else:
                            next_rng = last_mag
                            next_mag = last_mag // 2
                    else:
                        next_mag = rng // 2
                        next_rng = next_mag

                    if next_mag == 0:
                        if response_dir == -1:
                            converged.add(ref)
                            boundary.add(ref)
                        else:
                            correction.add(ref)
                            reached = _walk(my_dag, ref, 1)
                            for r in reached:
                                if r not in boundary and r not in new_proposals and not _is_shared(r):
                                    _process_proposal(r, 1, 1, new_proposals)
                    else:
                        # Walk with retry through confirmed shared territory
                        while next_mag > 0:
                            next_hop = response_dir * next_mag
                            reached = _walk(my_dag, ref, next_hop)
                            reached_new = {r for r in reached
                                           if r not in boundary
                                           and r not in new_proposals
                                           and not _is_shared(r)}
                            if reached_new:
                                for r in reached_new:
                                    _process_proposal(r, next_hop, next_rng, new_proposals)
                                break
                            reached_tracked = {r for r in reached if r in new_proposals}
                            if reached_tracked:
                                break  # covered by another path
                            # All reached refs are confirmed shared — narrow.
                            next_mag = next_mag // 2
                            if next_rng is not None:
                                next_rng = next_mag
                        else:
                            # Narrowed to 0 with all shared. Converge.
                            if response_dir == -1:
                                converged.add(ref)
                                boundary.add(ref)

                new_proposals = {}
                for ref, (last_hop, rng) in proposals.items():
                    _process_proposal(ref, last_hop, rng, new_proposals)

                proposals.clear()
                proposals.update(new_proposals)

                # --- 3. Build outgoing probe (non-converged proposals) ---
                out_probe = set(proposals.keys())

                records.append(TickRecord(
                    tick=tick, node=node,
                    proposals_before=proposals_before,
                    received_probe=set(recv_probe),
                    received_have=set(recv_have),
                    received_want=set(recv_want),
                    converged_this_tick=converged,
                    correction_this_tick=correction,
                    proposals_after=dict(proposals),
                    boundary=set(boundary),
                    sent_probe=out_probe,
                    sent_have=out_have,
                    sent_want=out_want,
                ))

            # Wire up messages for next tick
            a_probe_next = set()
            a_have_next = set()
            a_want_next = set()
            b_probe_next = set()
            b_have_next = set()
            b_want_next = set()

            for r in records[-2:]:  # last two records (A and B for this tick)
                if r.node == "A":
                    a_probe_next = r.sent_probe
                    a_have_next = r.sent_have
                    a_want_next = r.sent_want
                else:
                    b_probe_next = r.sent_probe
                    b_have_next = r.sent_have
                    b_want_next = r.sent_want

            a_probe = a_probe_next
            a_have = a_have_next
            a_want = a_want_next
            b_probe = b_probe_next
            b_have = b_have_next
            b_want = b_want_next

            # Check termination
            if not proposals_a and not proposals_b:
                break

        return records

    return (run_duplex_reconciliation,)


@app.cell
def _(dag_a, dag_b, dag_info, mo, run_duplex_reconciliation):
    trace = run_duplex_reconciliation(dag_a, dag_b)

    def _fmt_msg(probe, have, want):
        parts = []
        if probe: parts.append(f"probe=`{probe}`")
        if have: parts.append(f"have=`{have}`")
        if want: parts.append(f"want=`{want}`")
        return ", ".join(parts) if parts else "*(empty)*"

    def _fmt_state(proposals, boundary, converged=None, correction=None):
        # Format proposals as ref: (hop, range) for readability
        p_str = ", ".join(f"{r}:({h},{rn})" for r, (h, rn) in sorted(proposals.items())) if proposals else "{}"
        parts = [f"proposals=`{{{p_str}}}`"]
        if boundary: parts.append(f"boundary=`{boundary}`")
        if converged: parts.append(f"converged=`{converged}`")
        if correction: parts.append(f"correction=`{correction}`")
        return "<br>".join(parts)

    # Build table rows per tick
    a_by_tick = {r.tick: r for r in trace if r.node == "A"}
    b_by_tick = {r.tick: r for r in trace if r.node == "B"}
    all_ticks = sorted(set(a_by_tick) | set(b_by_tick))

    rows = []
    rows.append("| Tick | A state | A → B | A ← B | B state |")
    rows.append("|------|---------|-------|-------|---------|")

    # Bootstrap row (before tick 0)
    a0 = a_by_tick.get(0)
    b0 = b_by_tick.get(0)
    if a0 and b0:
        rows.append(
            f"| boot | {_fmt_state(a0.proposals_before, set())} "
            f"| {_fmt_msg(a0.sent_probe if a0 else set(), set(), set())} "
            f"| {_fmt_msg(b0.sent_probe if b0 else set(), set(), set())} "
            f"| {_fmt_state(b0.proposals_before, set())} |"
        )

    for tick in all_ticks:
        a = a_by_tick.get(tick)
        b = b_by_tick.get(tick)

        # A ← B: what A received (= what B sent last tick)
        a_recv = _fmt_msg(
            a.received_probe if a else set(),
            a.received_have if a else set(),
            a.received_want if a else set(),
        ) if a else ""

        # A → B: what A sends this tick
        a_send = _fmt_msg(
            a.sent_probe if a else set(),
            a.sent_have if a else set(),
            a.sent_want if a else set(),
        ) if a else ""

        a_state = _fmt_state(
            a.proposals_after if a else {},
            a.boundary if a else set(),
            a.converged_this_tick if a else None,
            a.correction_this_tick if a else None,
        ) if a else ""

        b_state = _fmt_state(
            b.proposals_after if b else {},
            b.boundary if b else set(),
            b.converged_this_tick if b else None,
            b.correction_this_tick if b else None,
        ) if b else ""

        rows.append(f"| {tick} | {a_state} | {a_send} | {a_recv} | {b_state} |")

    lines = ["## Trace\n"]
    lines.extend(rows)
    lines.append("")

    # Results
    a_records = [r for r in trace if r.node == "A"]
    b_records = [r for r in trace if r.node == "B"]

    for node, records, my_dag, peer_dag, only_mine_key in [
        ("A", a_records, dag_a, dag_b, "only_a"),
        ("B", b_records, dag_b, dag_a, "only_b"),
    ]:
        final = records[-1] if records else None
        if final and not final.proposals_after:
            bnd = final.boundary
            delta = my_dag.forward_walk(bnd)
            actual = [d for d in delta if not peer_dag.has(d)]
            wasted = [d for d in delta if peer_dag.has(d)]
            peer = "B" if node == "A" else "A"
            lines.append(f"### Result: {node} → {peer}")
            lines.append(f"**Boundary:** `{bnd}`")
            lines.append(f"**Forward walk (inclusive):** `{delta}`")
            lines.append(f"**Actual messages {peer} needs:** `{dag_info[only_mine_key]}`")
            if wasted:
                lines.append(f"**Wasted:** `{set(wasted)}`")
            else:
                lines.append(f"**Exact diff — no wasted messages.**")
            lines.append("")

    mo.md("\n".join(lines))
    return (trace,)


@app.cell
def _(mo, trace):
    _max_tick = max(r.tick for r in trace) if trace else 0
    tick_selector = mo.ui.slider(
        start=0,
        stop=max(_max_tick, 0),
        value=0,
        label="Tick",
        full_width=True,
    )
    tick_selector
    return (tick_selector,)


@app.cell
def _(dag_a, dag_b, dag_info, draw_dag, mo, tick_selector, trace):
    _tick = tick_selector.value
    _a_rec = [r for r in trace if r.node == "A" and r.tick == _tick]
    _b_rec = [r for r in trace if r.node == "B" and r.tick == _tick]
    _a = _a_rec[0] if _a_rec else None
    _b = _b_rec[0] if _b_rec else None

    _figs = []
    if _a:
        _fig_a = draw_dag(
            dag_a, f"Node A — Tick {_tick}",
            only_mine=dag_info["only_a"], only_theirs=dag_info["only_b"],
            active=set(_a.proposals_after.keys()), boundary=_a.boundary,
        )
        _figs.append(_fig_a)
        _a_p = ", ".join(f"{r}:({h},{rn})" for r, (h, rn) in sorted(_a.proposals_after.items()))
        _figs.append(mo.md(f"**A proposals:** `{{{_a_p}}}` | **boundary:** `{_a.boundary}`"))

    if _b:
        _fig_b = draw_dag(
            dag_b, f"Node B — Tick {_tick}",
            only_mine=dag_info["only_b"], only_theirs=dag_info["only_a"],
            active=set(_b.proposals_after.keys()), boundary=_b.boundary,
        )
        _figs.append(_fig_b)
        _b_p = ", ".join(f"{r}:({h},{rn})" for r, (h, rn) in sorted(_b.proposals_after.items()))
        _figs.append(mo.md(f"**B proposals:** `{{{_b_p}}}` | **boundary:** `{_b.boundary}`"))

    if not _figs:
        _figs.append(mo.md(f"*No records for tick {_tick}*"))

    mo.vstack(_figs)
    return


if __name__ == "__main__":
    app.run()
