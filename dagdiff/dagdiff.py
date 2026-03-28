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

    @dataclass
    class Msg:
        """A message in the DAG."""
        id: str
        parents: list[str] = field(default_factory=list)
        generation: int = 0

    class DAG:
        """Simple in-memory DAG of messages."""

        def __init__(self):
            self.msgs: dict[str, Msg] = {}  # id → Msg (incorporated)
            self.pending: set[str] = set()  # ids present but not incorporated

        def copy(self) -> "DAG":
            d = DAG()
            for m in self.msgs.values():
                d.msgs[m.id] = Msg(
                    id=m.id,
                    parents=list(m.parents),
                    generation=m.generation,
                )
            d.pending = set(self.pending)
            return d

        def add(self, id: str, parents: list[str]) -> Msg:
            gen = max((self.msgs[p].generation for p in parents), default=-1) + 1
            m = Msg(id=id, parents=parents, generation=gen)
            self.msgs[m.id] = m
            return m

        def add_pending(self, id: str) -> None:
            """Mark an id as pending (have bytes, not incorporated)."""
            self.pending.add(id)

        def has_incorporated(self, id: str) -> bool:
            return id in self.msgs

        def has_pending(self, id: str) -> bool:
            return id in self.pending

        def has(self, id: str) -> bool:
            return id in self.msgs or id in self.pending

        def frontier(self) -> set[str]:
            """Messages with no children."""
            has_child = set()
            for m in self.msgs.values():
                for p in m.parents:
                    has_child.add(p)
            return {id for id in self.msgs if id not in has_child}

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
        # B also has some of A's messages as pending (has bytes, not incorporated).
        # This simulates receiving a3 and a4 via push but missing their ancestors.
        dag_b.add_pending("a3")
        dag_b.add_pending("a4")

        info = {
            "shared": set(base.msgs.keys()),
            "only_a": set(dag_a.msgs.keys()) - set(base.msgs.keys()),
            "only_b": set(dag_b.msgs.keys()) - set(base.msgs.keys()),
            "pending_b": set(dag_b.pending),
        }
        return dag_a, dag_b, info

    dag_a, dag_b, dag_info = build_example()

    mo.md(f"""
    **Node A**: {len(dag_a.msgs)} messages, frontier = `{dag_a.frontier()}`

    **Node B**: {len(dag_b.msgs)} messages, frontier = `{dag_b.frontier()}`, pending = `{dag_b.pending}`

    **Shared**: {len(dag_info['shared'])} messages

    **Only A**: `{dag_info['only_a']}`

    **Only B**: `{dag_info['only_b']}`

    **Pending on B**: `{dag_info['pending_b']}` (B has bytes but not incorporated — ancestors missing)
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

    Binary search over DAG paths to find the **boundary** between shared and
    unshared regions. Each node runs the same algorithm simultaneously.
    **Proposals** (refs being probed) expand exponentially until overshooting,
    then narrow by halving until they pinpoint exact boundary refs.

    ### Messages

    Two message types:
    - **ProbeRequest**: `have` — "I have these refs, do you?"
    - **ProbeResponse**: `have`, `want`, `pending` — answers to a peer's ProbeRequest

    The three-way response separates two concerns:
    - **Boundary detection** needs `have` (incorporated, with ancestor guarantees) vs everything else
    - **Delta construction** needs to know what the peer already has bytes for (`have` + `pending`) to avoid redundant sends

    ### Per-peer state

    ```
    proposals: dict[ref → (direction, magnitude, range)]
        direction: +1 (toward frontier) or -1 (toward genesis)
        magnitude: step size (positive int)
        range:     None = expansion phase, int = narrowing phase

    hits:   set[ref]  — refs the peer has (incorporated)
    misses: set[ref]  — refs the peer doesn't have (want + pending for navigation)
    peer_has: set[ref] — refs the peer has bytes for (incorporated + pending, for delta filtering)
        Cached peer responses. Proposals landing on known refs resolve
        immediately without an extra round trip.
    ```

    ### Protocol

    **Bootstrap:** Send ProbeRequest with own frontier. Initial proposals: `direction = -1, magnitude = 1, range = None`.

    **Each tick** (process all messages in inbox):

    1. **Process ProbeRequests** — for each ProbeRequest from peer:
       - Cache `have` refs as hits (the peer has them).
       - Send ProbeResponse with `have`/`want`/`pending` for each ref.

    2. **Process ProbeResponses** — for each ProbeResponse from peer:
       - Cache `have` as hits, `want` and `pending` as misses (for boundary navigation).
       - Cache `have` and `pending` into `peer_has` (for delta filtering).

    3. **Update proposals** against the cache:

       *Compute the next step:*

       - **Expansion** (`range` is None):
         - Same direction → double: `magnitude × 2`.
         - Direction flips → begin narrowing: `range = magnitude`, `magnitude = range ÷ 2`.
       - **Narrowing** (`range` is set):
         - Halve: `magnitude = range ÷ 2`. Direction from response (hit → forward, miss → backward).

       *Apply the result:*

       - `magnitude = 0` → **converged**:
         - Miss → add ref to **boundary**.
         - Hit → **correction**: probe ref's children with `(dir=+1, mag=1, range=1)`.
           Child miss → boundary. Child hit → drop (shared).
       - `magnitude > 0` → **walk** `direction × magnitude` hops from ref.
         Reached refs become new proposals. If walk goes off the DAG, retry
         with shorter hops.
       - No response → carry forward unchanged (async tolerance).

       New proposals with cached responses are resolved immediately (recursive).

    4. **Send ProbeRequest** with all remaining proposals.

    **Termination:** All proposals empty → forward-walk from boundary (inclusive), filtering out refs in `peer_has` = delta.
    """)
    return


@app.cell
def _():
    from dataclasses import dataclass as _dataclass

    @_dataclass
    class _ProbeRequest:
        """Probe message: sender has these refs — does the peer?"""
        have: set

    @_dataclass
    class _ProbeResponse:
        """Reply to a ProbeRequest: three-way classification."""
        have: set     # incorporated (hit for DAGdiff, skip in delta)
        want: set     # unknown (miss for DAGdiff, include in delta)
        pending: set  # has bytes but not incorporated (miss for DAGdiff, skip in delta)

    @_dataclass
    class TickRecord:
        """What happened on one tick, from one node's perspective."""
        tick: int
        node: str                    # "A" or "B"
        # State at start of tick
        proposals_before: dict       # ref → (direction, magnitude, range)
        # Received
        received_probe: set          # from peer's ProbeRequest(s)
        received_have: set           # from peer's ProbeResponse(s)
        received_want: set           # from peer's ProbeResponse(s)
        received_pending: set        # from peer's ProbeResponse(s)
        # What happened
        converged_this_tick: set      # refs that converged (boundary misses)
        correction_this_tick: set     # refs that need correction step (boundary hits)
        # State after update
        proposals_after: dict        # ref → (direction, magnitude, range)
        boundary: set                # all converged boundary refs so far
        peer_has: set                # refs peer has bytes for (incorporated + pending)
        # Sent
        sent_probe: set              # our ProbeRequest
        sent_have: set               # our ProbeResponse
        sent_want: set               # our ProbeResponse
        sent_pending: set            # our ProbeResponse

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
                    nxt.update(dag.children_of({cid}))
                else:
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
        """Simulate proposal-based reconciliation between A and B
        using explicit request/response message passing."""

        # Per-node state: ref → (direction, magnitude, range)
        proposals_a = {f: (-1, 1, None) for f in dag_a.frontier()}
        proposals_b = {f: (-1, 1, None) for f in dag_b.frontier()}
        boundary_a: set[str] = set()
        boundary_b: set[str] = set()
        hits_a: set[str] = set()   # refs the peer confirmed having (incorporated)
        misses_a: set[str] = set() # refs the peer doesn't have usably (want + pending)
        hits_b: set[str] = set()
        misses_b: set[str] = set()
        peer_has_a: set[str] = set()  # refs peer has bytes for (incorporated + pending)
        peer_has_b: set[str] = set()
        records: list[TickRecord] = []

        # Bootstrap: each node sends a ProbeRequest with its frontier.
        # A's ProbeRequest goes to B's inbox; B's goes to A's inbox.
        inbox_a: list = [_ProbeRequest(have=set(proposals_b.keys()))]
        inbox_b: list = [_ProbeRequest(have=set(proposals_a.keys()))]

        for tick in range(max_ticks):
            outbox_a: list = []  # A sends these → B receives next tick
            outbox_b: list = []  # B sends these → A receives next tick

            for node, my_dag, proposals, boundary, hits, misses, peer_has, inbox, outbox in [
                ("A", dag_a, proposals_a, boundary_a, hits_a, misses_a, peer_has_a, inbox_a, outbox_a),
                ("B", dag_b, proposals_b, boundary_b, hits_b, misses_b, peer_has_b, inbox_b, outbox_b),
            ]:
                proposals_before = dict(proposals)
                requests = [m for m in inbox if isinstance(m, _ProbeRequest)]
                responses = [m for m in inbox if isinstance(m, _ProbeResponse)]

                # --- 1. Process ProbeRequests → send ProbeResponse ---
                all_recv_probe = set()
                out_have = set()
                out_want = set()
                out_pending = set()
                for req in requests:
                    all_recv_probe |= req.have
                    for ref in req.have:
                        hits.add(ref)  # probe refs are implicit hits (peer has them)
                        misses.discard(ref)
                        peer_has.add(ref)
                        if my_dag.has_incorporated(ref):
                            out_have.add(ref)
                        elif my_dag.has_pending(ref):
                            out_pending.add(ref)
                        else:
                            out_want.add(ref)
                if out_have or out_want or out_pending:
                    outbox.append(_ProbeResponse(have=out_have, want=out_want, pending=out_pending))

                # --- 2. Process ProbeResponses → update cache ---
                all_recv_have = set()
                all_recv_want = set()
                all_recv_pending = set()
                for resp in responses:
                    all_recv_have |= resp.have
                    all_recv_want |= resp.want
                    all_recv_pending |= resp.pending
                    # hits = incorporated only (for DAGdiff navigation)
                    hits.update(resp.have)
                    hits -= resp.want
                    hits -= resp.pending
                    # misses = want + pending (for DAGdiff navigation)
                    misses.update(resp.want)
                    misses.update(resp.pending)
                    misses -= resp.have
                    # peer_has = have + pending (for delta filtering)
                    peer_has.update(resp.have)
                    peer_has.update(resp.pending)

                # --- 3. Update proposals against cache ---
                converged = set()
                correction = set()

                def _process_proposal(ref, direction, magnitude, rng, new_proposals):
                    """Process a single proposal against hits/misses. May recurse
                    for new proposals that have cached responses."""
                    is_hit = ref in hits
                    is_miss = ref in misses
                    if not is_hit and not is_miss:
                        new_proposals[ref] = (direction, magnitude, rng)
                        return

                    resp_dir = 1 if is_hit else -1

                    if rng is None:
                        # Expansion phase
                        if direction == resp_dir:
                            next_mag = magnitude * 2
                            next_rng = None
                        else:
                            next_rng = magnitude
                            next_mag = magnitude // 2
                    else:
                        # Narrowing phase
                        next_mag = rng // 2
                        next_rng = next_mag

                    if next_mag == 0:
                        # Converged
                        if is_miss:
                            converged.add(ref)
                            boundary.add(ref)
                        else:
                            # Correction: probe children to find exact boundary
                            correction.add(ref)
                            for r in _walk(my_dag, ref, 1):
                                if r not in boundary and r not in new_proposals:
                                    _process_proposal(r, 1, 1, 1, new_proposals)
                    else:
                        # Walk; shorten hop if it goes off the DAG
                        reached = set()
                        walk_mag = next_mag
                        while walk_mag > 0 and not reached:
                            reached = _walk(my_dag, ref, resp_dir * walk_mag)
                            if not reached:
                                walk_mag //= 2
                                if next_rng is not None:
                                    next_rng = walk_mag
                        if reached:
                            for r in reached:
                                if r not in boundary and r not in new_proposals:
                                    _process_proposal(r, resp_dir, walk_mag, next_rng, new_proposals)
                        elif is_miss:
                            converged.add(ref)
                            boundary.add(ref)

                new_proposals = {}
                for ref, (direction, magnitude, rng) in proposals.items():
                    _process_proposal(ref, direction, magnitude, rng, new_proposals)
                proposals.clear()
                proposals.update(new_proposals)

                # --- 4. Send ProbeRequest for remaining proposals ---
                out_probe = set(proposals.keys())
                if out_probe:
                    outbox.append(_ProbeRequest(have=out_probe))

                records.append(TickRecord(
                    tick=tick, node=node,
                    proposals_before=proposals_before,
                    received_probe=all_recv_probe,
                    received_have=all_recv_have,
                    received_want=all_recv_want,
                    received_pending=all_recv_pending,
                    converged_this_tick=converged,
                    correction_this_tick=correction,
                    proposals_after=dict(proposals),
                    boundary=set(boundary),
                    peer_has=set(peer_has),
                    sent_probe=out_probe,
                    sent_have=out_have,
                    sent_want=out_want,
                    sent_pending=out_pending,
                ))

            # Deliver: A's outbox → B's next inbox, and vice versa
            inbox_a = outbox_b
            inbox_b = outbox_a

            if not proposals_a and not proposals_b:
                break

        return records

    return (run_duplex_reconciliation,)


@app.cell
def _(dag_a, dag_b, dag_info, mo, run_duplex_reconciliation):
    trace = run_duplex_reconciliation(dag_a, dag_b)

    def _fmt_msg(probe, have, want, pending=None):
        parts = []
        if probe: parts.append(f"**req** `{probe}`")
        if have or want or pending:
            r = []
            if have: r.append(f"have=`{have}`")
            if want: r.append(f"want=`{want}`")
            if pending: r.append(f"pending=`{pending}`")
            parts.append(f"**resp** {', '.join(r)}")
        return "<br>".join(parts) if parts else "*(empty)*"

    def _fmt_state(proposals, boundary, peer_has=None, converged=None, correction=None):
        p_str = ", ".join(f"{r}:({d},{m},{rn})" for r, (d, m, rn) in sorted(proposals.items())) if proposals else "{}"
        parts = [f"proposals=`{{{p_str}}}`"]
        if boundary: parts.append(f"boundary=`{boundary}`")
        if peer_has: parts.append(f"peer_has=`{peer_has}`")
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

        a_recv = _fmt_msg(
            a.received_probe if a else set(),
            a.received_have if a else set(),
            a.received_want if a else set(),
            a.received_pending if a else set(),
        ) if a else ""

        a_send = _fmt_msg(
            a.sent_probe if a else set(),
            a.sent_have if a else set(),
            a.sent_want if a else set(),
            a.sent_pending if a else set(),
        ) if a else ""

        a_state = _fmt_state(
            a.proposals_after if a else {},
            a.boundary if a else set(),
            a.peer_has if a else None,
            a.converged_this_tick if a else None,
            a.correction_this_tick if a else None,
        ) if a else ""

        b_state = _fmt_state(
            b.proposals_after if b else {},
            b.boundary if b else set(),
            b.peer_has if b else None,
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
            full_walk = my_dag.forward_walk(bnd)
            filtered_delta = [d for d in full_walk if d not in final.peer_has]
            skipped = [d for d in full_walk if d in final.peer_has]
            actual_needed = [d for d in full_walk if not peer_dag.has(d)]
            wasted = [d for d in filtered_delta if peer_dag.has(d)]
            peer = "B" if node == "A" else "A"
            lines.append(f"### Result: {node} → {peer}")
            lines.append(f"**Boundary:** `{bnd}`")
            lines.append(f"**Forward walk (inclusive):** `{full_walk}` ({len(full_walk)} msgs)")
            lines.append(f"**Skipped (peer has bytes):** `{set(skipped)}` ({len(skipped)} msgs)")
            lines.append(f"**Delta sent:** `{filtered_delta}` ({len(filtered_delta)} msgs)")
            lines.append(f"**Actual messages {peer} needs:** `{dag_info[only_mine_key]}`")
            if wasted:
                lines.append(f"**Wasted (sent but peer has):** `{set(wasted)}`")
            else:
                lines.append(f"**No wasted messages in filtered delta.**")
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
        _a_p = ", ".join(f"{r}:({d},{m},{rn})" for r, (d, m, rn) in sorted(_a.proposals_after.items()))
        _figs.append(mo.md(f"**A proposals:** `{{{_a_p}}}` | **boundary:** `{_a.boundary}`"))

    if _b:
        _fig_b = draw_dag(
            dag_b, f"Node B — Tick {_tick}",
            only_mine=dag_info["only_b"], only_theirs=dag_info["only_a"],
            active=set(_b.proposals_after.keys()), boundary=_b.boundary,
        )
        _figs.append(_fig_b)
        _b_p = ", ".join(f"{r}:({d},{m},{rn})" for r, (d, m, rn) in sorted(_b.proposals_after.items()))
        _figs.append(mo.md(f"**B proposals:** `{{{_b_p}}}` | **boundary:** `{_b.boundary}`"))

    if not _figs:
        _figs.append(mo.md(f"*No records for tick {_tick}*"))

    mo.vstack(_figs)
    return


if __name__ == "__main__":
    app.run()
