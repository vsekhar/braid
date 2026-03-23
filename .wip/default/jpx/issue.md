---
priority: p2
type: task
created: 2026-03-23T14:18:37-04:00
updated: 2026-03-23T14:18:37-04:00
---

# Convert swarm_viz.py to Jupyter notebook

## Objective

Convert `analysis/swarm_viz.py` into a Jupyter notebook (`analysis/swarm_viz.ipynb`) so the visualization can be iterated on interactively.

## Context

`analysis/swarm_viz.py` is a standalone PEP 723 script that parses `swarm.log` and produces 3 time-series charts (incorporated, pending, wanted) with one line per 10 nodes. Converting to a notebook enables interactive exploration — tweaking chart parameters, filtering nodes, zooming into time ranges, etc.

## Source

The current script (`analysis/swarm_viz.py`) has this structure:

1. **Imports and constants** — matplotlib, re, datetime, defaultdict
2. **Regex** — matches `received message` log lines, extracts timestamp, node, incorporated, pending, wanted
3. **`parse_log(path)`** — returns `{node: [(elapsed_seconds, incorporated, pending, wanted), ...]}`
4. **`plot(records)`** — creates 3-panel subplot (one per field), one line per node, saves PNG
5. **`main()`** — CLI entry point with optional log path argument

## Approach

Create `analysis/swarm_viz.ipynb` with cells split along natural boundaries:

1. **Metadata/dependencies cell** — document required packages (matplotlib). Use `uv` to manage the notebook environment.
2. **Imports cell** — all imports
3. **Configuration cell** — log file path, regex pattern (easy to tweak)
4. **Parsing cell** — `parse_log()` function + call it, print summary stats
5. **Visualization cell(s)** — one cell per chart or a single cell with all 3 subplots

The notebook should be runnable with `uv run jupyter lab` or similar. Remove `swarm_viz.py` after conversion since the notebook replaces it.

## Acceptance Criteria

- [ ] `analysis/swarm_viz.ipynb` exists and produces the same 3-panel chart
- [ ] Notebook cells are logically organized for interactive use
- [ ] `analysis/swarm_viz.py` is removed
- [ ] Notebook runs cleanly with `uv`-managed dependencies
