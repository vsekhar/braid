#!/usr/bin/env python3
# /// script
# requires-python = ">=3.13"
# dependencies = [
#     "matplotlib>=3.10.8",
# ]
# ///
"""Parse swarm.log and plot incorporated, pending, wanted over time per node."""

import re
import sys
from collections import defaultdict
from datetime import datetime

import matplotlib.pyplot as plt

LOG_FILE = "../swarm.log"
OUTPUT_FILE = "swarm_state.png"

LINE_RE = re.compile(
    r'time=(\S+)\s+level=\S+\s+msg="received message"\s+'
    r'node=([0-9a-f]+)\s+ref=[0-9a-f]+\s+'
    r'incorporated=(\d+)\s+pending=(\d+)\s+wanted=(\d+)'
)


def parse_log(path):
    """Return {node: [(seconds_from_start, incorporated, pending, wanted), ...]}."""
    records = defaultdict(list)
    t0 = None

    with open(path) as f:
        for line in f:
            m = LINE_RE.search(line)
            if not m:
                continue
            ts_str, node, inc, pend, want = m.groups()
            ts = datetime.fromisoformat(ts_str)
            if t0 is None:
                t0 = ts
            elapsed = (ts - t0).total_seconds()
            records[node].append((elapsed, int(inc), int(pend), int(want)))

    return records


def plot(records):
    fields = ["incorporated", "pending", "wanted"]
    fig, axes = plt.subplots(3, 1, figsize=(14, 10), sharex=True)

    for ax, field, idx in zip(axes, fields, [1, 2, 3]):
        for node in sorted(records):
            data = records[node]
            t = [r[0] for r in data]
            v = [r[idx] for r in data]
            ax.plot(t, v, label=node, linewidth=0.7, alpha=0.85)
        ax.set_ylabel(field)
        ax.legend(fontsize=7, ncol=5, loc="upper left")
        ax.grid(True, alpha=0.3)

    axes[-1].set_xlabel("time (seconds from start)")
    axes[0].set_title("Swarm node state over time")
    fig.tight_layout()
    fig.savefig(OUTPUT_FILE, dpi=150)
    print(f"Saved to {OUTPUT_FILE}")


def main():
    path = sys.argv[1] if len(sys.argv) > 1 else LOG_FILE
    records = parse_log(path)
    if not records:
        print(f"No 'received message' lines found in {path}", file=sys.stderr)
        sys.exit(1)
    print(f"Parsed {sum(len(v) for v in records.values())} entries across {len(records)} nodes")
    plot(records)


if __name__ == "__main__":
    main()
