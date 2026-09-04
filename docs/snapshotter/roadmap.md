# Snapshotter Roadmap / Release Scoping

> Build-order and backlog tracking for `snapshotter`. **No code changes live here**; this
> file only records issue status and pick-up order.
>
> Last updated: 2026-09-04T15:05:00Z

## Backlog status snapshot

| Issue | Title | Status | Notes |
|-------|-------|--------|-------|
| 01 | Event Ledger Core State | `completed` | Foundation |
| 02 | Path Normalization & Ignoring | `completed` | Foundation |
| 03 | Interactive TUI File History Viewer | `completed` | Merged to `main` (`cb9440c`) |
| 04 | Snapshot Rollbacks & Restorations | `completed` | Merged to `main` (`90de1b2`) |
| 05 | Action Lineage & Unique Identifiers | `completed` | Implemented — lineage schema + reducer + tests |
| 06 | Represent newly-ignored files as IGNORED, not DELETE | `completed` | Code already merged to `main`, all ACs `[x]` |
| 07 | End-to-end engine integration tests | `completed` | |
| 08 | CLI snapshot header dominant-action label | `proposed` | Output-only change; independent |
| 09 | Engine diff can miss equal-length modifications within timestamp granularity | `proposed` | Real engine correctness bug; independent |

Foundation + lineage + restores + TUI are all landed: **01–07 are done**. The remaining
backlog is **08** and **09**. Nothing is hard-blocked; both are implementable against the
current schema and are independent of each other.

## Pick-up order for next session

1. **`09` — engine diff modtime short-circuit bug.** This is the highest-value item: a
   genuine correctness bug where the `size == && ModTime.Equal` fast path in
   `internal/store/engine.go` can silently drop an *equal-length* modification written
   within filesystem timestamp granularity. Fixing it (likely by trusting the hash as the
   authoritative signal, or only trusting the mtime fast path when the recorded mtime is
   sufficiently old) is a correctness improvement and should land before further consumers
   of the change stream are built on top of it. Include the requested reproduction test.
2. **`08` — CLI snapshot header dominant-action label.** Small, output-only change in
   `snapshotter.go`. Decides/confirms the parenthetical label precedence for mixed-action
   snapshots (`(modified)` / `(moved)`, and how `CREATE` / `DELETE` / `IGNORED`-only
   snapshots read). Independent of `09` and can be slotted in at any point.
