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
    # Swarm Node State Visualization

    Parse `swarm.log` and plot **incorporated**, **pending**, and **wanted** over time, one line per node.
    """)
    return


@app.cell
def _():
    import re
    from collections import defaultdict
    from datetime import datetime
    from pathlib import Path

    import matplotlib.pyplot as plt

    return Path, datetime, defaultdict, plt, re


@app.cell
def _(Path, mo, re):
    _notebook_dir = Path(mo.notebook_dir())
    LOG_FILE = str(_notebook_dir / "../swarm2.log")
    OUT_DIR = _notebook_dir

    RECEIVED_RE = re.compile(
        r'time=(\S+)\s+level=\S+\s+msg="received message"\s+'
        r'node=([0-9a-f]+)\s+ref=[0-9a-f]+\s+'
        r'incorporated=(\d+)\s+pending=(\d+)\s+wanted=(\d+)'
    )

    CREATED_RE = re.compile(
        r'time=(\S+)\s+level=\S+\s+msg="created message"\s+'
        r'node=([0-9a-f]+)\s+ref=[0-9a-f]+\s+peers=(\d+)\s+incorporated=(\d+)'
    )

    RESOLVE_RE = re.compile(
        r'time=(\S+)\s+level=\S+\s+msg="resolving wanted"\s+'
        r'node=([0-9a-f]+)\s+peer=([0-9a-f]+)\s+sending=(\d+)'
    )

    WANTED_REQ_RE = re.compile(
        r'time=(\S+)\s+level=\S+\s+msg="sending wanted request"\s+'
        r'node=([0-9a-f]+)\s+peer=([0-9a-f]+)\s+wanted=(\d+)\s+frontier=(\d+)'
    )
    return (
        CREATED_RE,
        LOG_FILE,
        OUT_DIR,
        RECEIVED_RE,
        RESOLVE_RE,
        WANTED_REQ_RE,
    )


@app.cell
def _(
    CREATED_RE,
    LOG_FILE,
    RECEIVED_RE,
    RESOLVE_RE,
    WANTED_REQ_RE,
    datetime,
    defaultdict,
):
    def parse_timestamp(ts_str, t0):
        ts = datetime.fromisoformat(ts_str)
        if t0 is None:
            t0 = ts
        return ((ts - t0).total_seconds(), t0)

    def parse_log(path):
        """Parse all event types from the log."""
        received = defaultdict(list)   # node -> [(t, incorporated, pending, wanted)]
        created = defaultdict(list)    # node -> [(t, peers, incorporated)]
        resolved = defaultdict(list)   # node -> [(t, peer, sending)]
        wanted_reqs = defaultdict(list) # node -> [(t, peer, wanted_count)]
        t0 = None
        with open(path) as f:
            for line in f:
                m = RECEIVED_RE.search(line)
                if m:
                    ts_str, _node, inc, pend, want = m.groups()
                    elapsed, t0 = parse_timestamp(ts_str, t0)
                    received[_node].append((elapsed, int(inc), int(pend), int(want)))
                    continue
                m = CREATED_RE.search(line)
                if m:
                    ts_str, _node, peers, inc = m.groups()
                    elapsed, t0 = parse_timestamp(ts_str, t0)
                    created[_node].append((elapsed, int(peers), int(inc)))
                    continue
                m = RESOLVE_RE.search(line)
                if m:
                    ts_str, _node, _peer, sending = m.groups()
                    elapsed, t0 = parse_timestamp(ts_str, t0)
                    resolved[_node].append((elapsed, _peer, int(sending)))
                    continue
                m = WANTED_REQ_RE.search(line)
                if m:
                    ts_str, _node, _peer, wanted_count, _frontier = m.groups()
                    elapsed, t0 = parse_timestamp(ts_str, t0)
                    wanted_reqs[_node].append((elapsed, _peer, int(wanted_count)))
        return (received, created, resolved, wanted_reqs)
    received, created, resolved, wanted_reqs = parse_log(LOG_FILE)
    print(f'Received:     {sum((len(v) for v in received.values()))} entries across {len(received)} nodes')
    print(f'Created:      {sum((len(v) for v in created.values()))} entries across {len(created)} nodes')
    print(f'Resolved:     {sum((len(v) for v in resolved.values()))} entries across {len(resolved)} nodes')
    print(f'Wanted reqs:  {sum((len(v) for v in wanted_reqs.values()))} entries across {len(wanted_reqs)} nodes')
    return created, received, resolved, wanted_reqs


@app.cell
def _(OUT_DIR, plt, received):
    fields = ['incorporated', 'pending', 'wanted']
    _fig, axes = plt.subplots(3, 1, figsize=(14, 10), sharex=True)
    for _ax, field, idx in zip(axes, fields, [1, 2, 3]):
        for _node in sorted(received):
            _data = received[_node]
            _t = [r[0] for r in _data]
            v = [r[idx] for r in _data]
            _ax.plot(_t, v, label=_node, linewidth=0.7, alpha=0.85)
        _ax.set_ylabel(field)
        _ax.legend(fontsize=7, ncol=5, loc='upper left')
        _ax.grid(True, alpha=0.3)
    axes[-1].set_xlabel('time (seconds from start)')
    axes[0].set_title('Swarm node state over time')
    _fig.tight_layout()
    _fig.savefig(OUT_DIR / 'out_state.png', dpi=100)
    _fig
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md(r"""
    ## Incorporation rate vs. creation rate

    Compares how fast each node incorporates messages (derivative of `incorporated`) against how fast messages are created network-wide. If incorporation rate drops below creation rate, the node is falling behind.
    """)
    return


@app.cell
def _(OUT_DIR, created, plt, received):
    WINDOW = 10  # seconds for rolling rate
    T_START = 15  # skip startup transient (seconds)

    def rolling_rate(times, values, window=WINDOW):
        """Compute rate (delta values / delta time) over a rolling window."""
        rate_t, rate_v = ([], [])
        j = 0
        for i in range(len(times)):
            while j < len(times) and times[j] < times[i] - window:
                j += 1
            dt = times[i] - times[j] if i != j else 1.0
            rate_t.append(times[i])
            rate_v.append((values[i] - values[j]) / dt if dt > 0 else 0)
        return (rate_t, rate_v)
    all_create_times = sorted((_t for _node in created.values() for _t, _, _ in _node))
    # Network-wide creation rate (all nodes combined)
    create_rate_t, create_rate_v = ([], [])
    j = 0
    for i in range(len(all_create_times)):
        while j < len(all_create_times) and all_create_times[j] < all_create_times[i] - WINDOW:
            j += 1
        dt = all_create_times[i] - all_create_times[j] if i != j else 1.0
        create_rate_t.append(all_create_times[i])
        create_rate_v.append((i - j) / dt if dt > 0 else 0)
    _fig, _ax = plt.subplots(figsize=(14, 5))
    for _node in sorted(received):
        _data = received[_node]
        _t = [r[0] for r in _data]
        inc = [r[1] for r in _data]
        rt, rv = rolling_rate(_t, inc)
        _ax.plot(rt, rv, label=_node, linewidth=0.7, alpha=0.8)
    _ax.plot(create_rate_t, create_rate_v, 'k--', linewidth=1.5, label='network creation rate', alpha=0.7)
    _ax.set_xlim(left=T_START)
    _ax.set_ylim(bottom=0, top=50)
    _ax.set_ylabel(f'messages/sec ({WINDOW}s window)')
    _ax.set_xlabel('time (seconds from start)')
    _ax.set_title('Incorporation rate per node vs. network creation rate')
    _ax.legend(fontsize=7, ncol=5, loc='upper left')
    _ax.grid(True, alpha=0.3)
    _fig.tight_layout()
    _fig.savefig(OUT_DIR / 'out_rate.png', dpi=100)
    _fig
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md(r"""
    ## Resolve batch sizes over time

    Each "resolving wanted" event sends a batch of messages to a peer. If batches are hitting the `maxResolve=1000` cap, the node can't send enough messages per resolve cycle to keep up.
    """)
    return


@app.cell
def _(OUT_DIR, plt, resolved):
    MAX_RESOLVE = 5000  # from store.go
    _fig, _ax = plt.subplots(figsize=(14, 4))
    for _node in sorted(resolved):
        _data = resolved[_node]
        _t = [r[0] for r in _data]
        _sending = [r[2] for r in _data]
        _ax.scatter(_t, _sending, label=_node, s=8, alpha=0.6)
    _ax.axhline(y=MAX_RESOLVE, color='r', linestyle='--', linewidth=1, label=f'maxResolve={MAX_RESOLVE}')
    _ax.set_ylabel('messages sent (sending)')
    _ax.set_xlabel('time (seconds from start)')
    _ax.set_title('Resolve batch sizes over time')
    _ax.legend(fontsize=7, ncol=5, loc='upper left')
    _ax.grid(True, alpha=0.3)
    _fig.tight_layout()
    _fig.savefig(OUT_DIR / 'out_resolve.png', dpi=100)
    _fig
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md(r"""
    ## Per-node connection count at spike time

    Shows how many peers each node is connected to over time. Nodes with fewer connections may propagate slower and accumulate more pending/wanted messages.
    """)
    return


@app.cell
def _(OUT_DIR, created, plt):
    # Peer count: use the 'peers' field from "created message" as a proxy for
    # how many connections a node has at each message creation event.
    _fig, _ax = plt.subplots(figsize=(14, 4))
    for _node in sorted(created):
        _data = created[_node]
        _t = [r[0] for r in _data]
        peers = [r[1] for r in _data]
        _ax.plot(_t, peers, label=_node, linewidth=0.7, alpha=0.8)
    _ax.set_ylabel('peers (at message creation)')
    _ax.set_xlabel('time (seconds from start)')
    _ax.set_title('Peer count per node over time')
    _ax.legend(fontsize=7, ncol=5, loc='upper left')
    _ax.grid(True, alpha=0.3)
    _fig.tight_layout()
    _fig.savefig(OUT_DIR / 'out_peers.png', dpi=100)
    _fig
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md(r"""
    ## Behind vs. caught-up node classification

    Classify nodes as "behind" or "caught up" based on whether their pending count exceeds a threshold
    in the second half of the run. Then check whether behind nodes are asking other behind nodes for help.
    """)
    return


@app.cell
def _(OUT_DIR, plt, received, resolved, wanted_reqs):
    # Classify nodes: "behind" if their average pending in the last half exceeds threshold
    BEHIND_THRESHOLD = 200
    all_nodes = sorted(received.keys())
    max_t = max(r[0] for node in received.values() for r in node)
    half_t = max_t / 2

    node_status = {}
    for _node in all_nodes:
        late_pending = [r[2] for r in received[_node] if r[0] > half_t]
        avg_pending = sum(late_pending) / len(late_pending) if late_pending else 0
        node_status[_node] = 'behind' if avg_pending > BEHIND_THRESHOLD else 'caught up'

    behind_nodes = {n for n, s in node_status.items() if s == 'behind'}
    caught_up_nodes = {n for n, s in node_status.items() if s == 'caught up'}

    # For each wanted request from a behind node, check if the peer it asked is also behind
    ask_behind = []  # (t, node) — asked a behind peer
    ask_caught = []  # (t, node) — asked a caught-up peer
    for _node in behind_nodes:
        for _t, _peer, _count in wanted_reqs.get(_node, []):
            if _t > half_t:
                if _peer in behind_nodes:
                    ask_behind.append((_t, _node))
                else:
                    ask_caught.append((_t, _node))

    total_asks = len(ask_behind) + len(ask_caught)
    pct_behind = 100 * len(ask_behind) / total_asks if total_asks > 0 else 0

    # For resolve responses TO behind nodes, check if responder was behind or caught-up
    resolve_from_behind = []   # (t, sending) — behind node resolved for a behind requester
    resolve_from_caught = []   # (t, sending) — caught-up node resolved for a behind requester

    # "resolving wanted" is logged by the RESPONDER, with the peer being the REQUESTER
    for _responder in all_nodes:
        for _t, _requester, _sending in resolved.get(_responder, []):
            if _t > half_t and _requester in behind_nodes:
                if _responder in behind_nodes:
                    resolve_from_behind.append((_t, _sending))
                else:
                    resolve_from_caught.append((_t, _sending))

    _fig, _axes = plt.subplots(1, 2, figsize=(14, 5))

    # Left: pie chart of who behind nodes asked
    _axes[0].pie(
        [len(ask_behind), len(ask_caught)],
        labels=[f'Asked behind peer\n({len(ask_behind)})', f'Asked caught-up peer\n({len(ask_caught)})'],
        colors=['#e74c3c', '#2ecc71'],
        autopct='%1.0f%%',
        startangle=90
    )
    _axes[0].set_title(f'Who do behind nodes ask for help?\n(second half, {total_asks} requests)')

    # Right: box plot of resolve batch sizes by responder type
    _bp_data = []
    _bp_labels = []
    if resolve_from_behind:
        _bp_data.append([s for _, s in resolve_from_behind])
        _bp_labels.append(f'Behind responder\n(n={len(resolve_from_behind)})')
    if resolve_from_caught:
        _bp_data.append([s for _, s in resolve_from_caught])
        _bp_labels.append(f'Caught-up responder\n(n={len(resolve_from_caught)})')
    if _bp_data:
        _axes[1].boxplot(_bp_data, labels=_bp_labels)
    _axes[1].set_ylabel('messages sent per resolve')
    _axes[1].set_title('Resolve batch size by responder status')
    _axes[1].grid(True, alpha=0.3)

    _fig.suptitle(
        f'Behind nodes: {", ".join(sorted(behind_nodes))} | '
        f'Caught up: {", ".join(sorted(caught_up_nodes))}',
        fontsize=8, y=0.02
    )
    _fig.tight_layout()
    _fig.subplots_adjust(bottom=0.1)
    _fig.savefig(OUT_DIR / 'out_bifurcation.png', dpi=100)
    _fig
    return


@app.cell(hide_code=True)
def _(mo):
    mo.md(r"""
    ## Wanted request timeline: who asks whom

    Shows each wanted request from a behind node as a dot, colored by whether the peer asked
    was also behind (red) or caught up (green). Wasted cycles (asking a behind peer) cluster
    together, showing how behind nodes get stuck asking each other.
    """)
    return


@app.cell
def _(OUT_DIR, plt, received, wanted_reqs):
    BEHIND_THRESHOLD_2 = 200
    _all_nodes = sorted(received.keys())
    _max_t = max(r[0] for node in received.values() for r in node)
    _half_t = _max_t / 2

    _node_status = {}
    for _n in _all_nodes:
        _late_pending = [r[2] for r in received[_n] if r[0] > _half_t]
        _avg_pending = sum(_late_pending) / len(_late_pending) if _late_pending else 0
        _node_status[_n] = 'behind' if _avg_pending > BEHIND_THRESHOLD_2 else 'caught up'
    _behind = {n for n, s in _node_status.items() if s == 'behind'}

    _fig, _ax = plt.subplots(figsize=(14, 5))
    _y_map = {n: i for i, n in enumerate(sorted(_behind))}

    for _n in sorted(_behind):
        for _t, _peer, _count in wanted_reqs.get(_n, []):
            _color = '#e74c3c' if _peer in _behind else '#2ecc71'
            _ax.scatter(_t, _y_map[_n], c=_color, s=max(3, _count / 5), alpha=0.5)

    _ax.set_yticks(list(_y_map.values()))
    _ax.set_yticklabels(list(_y_map.keys()), fontsize=8)
    _ax.set_xlabel('time (seconds from start)')
    _ax.set_ylabel('behind node')
    _ax.set_title('Wanted requests from behind nodes (red=asked behind peer, green=asked caught-up peer)')
    _ax.grid(True, alpha=0.3)

    # Legend
    from matplotlib.lines import Line2D
    _ax.legend(
        [Line2D([0], [0], marker='o', color='w', markerfacecolor='#e74c3c', markersize=8),
         Line2D([0], [0], marker='o', color='w', markerfacecolor='#2ecc71', markersize=8)],
        ['Asked behind peer', 'Asked caught-up peer'],
        fontsize=8, loc='upper left'
    )
    _fig.tight_layout()
    _fig.savefig(OUT_DIR / 'out_wanted_timeline.png', dpi=100)
    _fig
    return


if __name__ == "__main__":
    app.run()
