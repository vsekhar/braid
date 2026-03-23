---
priority: p2
type: task
created: 2026-03-22T21:45:05-04:00
updated: 2026-03-22T21:45:05-04:00
---

# Visualize swarm node state over time from swarm.log

## Objective

Create a standalone visualization script that parses `swarm.log` and produces three time-series charts — one each for `incorporated`, `pending`, and `wanted` — with one line per node (10 nodes total).

## Context

`swarm.log` contains structured log output from 10 concurrently-running braid nodes. Lines with `msg="received message"` report three state fields (`incorporated`, `pending`, `wanted`) along with a timestamp and node ID. The goal is to visualize how these values evolve over time across all nodes to understand convergence behavior.

This is standalone analysis work, separate from the main Go codebase.

## Data Format

Each relevant log line looks like:

```
time=2026-03-22T21:36:43.770-04:00 level=INFO msg="received message" node=c7dacd7f ref=6a6261bd incorporated=2 pending=0 wanted=0
```

- **Timestamps**: ISO 8601 with fractional seconds and timezone (`time=...`)
- **Node IDs**: 8-char hex strings (`node=...`), 10 unique nodes
- **Fields**: `incorporated` (integer, grows to ~3366), `pending` (0–22), `wanted` (0–26)
- **Volume**: ~30,184 `received message` lines across ~7 minutes of runtime
- **Log file path**: `swarm.log` (project root)

## Location

Create a new script in the project root, e.g. `analysis/swarm_viz.py` or similar. Keep it self-contained.

## Approach

- Use Python with matplotlib (or similar plotting library)
- Filter lines where `msg="received message"`
- Parse timestamp, node ID, and the three integer fields from each line
- Produce 3 separate charts (subplots or separate figures):
  - **Chart 1**: `incorporated` over time, one line per node
  - **Chart 2**: `pending` over time, one line per node
  - **Chart 3**: `wanted` over time, one line per node
- Each chart should have a legend mapping line colors to node IDs (short hex)
- X-axis: time (seconds from start or absolute timestamps)
- Save output to a file (PNG or PDF) and/or display interactively

## Acceptance Criteria

- [ ] Script parses `swarm.log` and extracts all `received message` lines
- [ ] Produces 3 charts (incorporated, pending, wanted) over time
- [ ] Each chart has 10 lines, one per node, with a legend
- [ ] Charts are readable and clearly labeled
- [ ] Script runs standalone (no dependency on project Go code)
