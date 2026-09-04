---
id: "08"
title: "CLI snapshot header should reflect the dominant action kind"
module: "snapshotter"
status: "proposed"
branch: ""
created: "2026-09-04T13:00:00Z"
updated: "2026-09-04T13:00:00Z"
blocked_by: []
---

# Issue 08: CLI snapshot header should reflect the dominant action kind

## Description
The CLI (`snapshotter.go`) always prints the same header line when a snapshot
has changes:

```
Snapshot created: <event ID>
```

This tells the user nothing about what the snapshot actually did. When the
snapshot contains modifications the header should instead say
`Snapshot created (modified)`; when it reflects a move it should say
`Snapshot created (moved)`. The event ID is still shown on the line.

This is **logging/output wording only** — leave all other behaviour as is. No
engine, ledger, state, diff, or file-layout changes are part of this issue.

## Desired output
A single header line whose parenthetical label reflects the dominant/preferred
action kind in the event, e.g.:

```
Snapshot created (modified): 01M1P74NPYXMVN19ZRKW8ABKV8
```

and, for a move:

```
Snapshot created (moved): 01M1P74NQ505MHBX8W1BBPKQ8J
```

The per-line change listing (`[+]`, `[~]`, `[-]`, `[i]`, `[>]`) is unchanged.

## Acceptance Criteria
- [ ] When a snapshot's changes are only modifications, the header reads `Snapshot created (modified): <ID>` instead of the generic `Snapshot created: <ID>`.
- [ ] When a snapshot's changes are only moves, the header reads `Snapshot created (moved): <ID>`.
- [ ] The event ID is still present on the header line.
- [ ] The existing per-change output lines (`[+]`, `[~]`, `[-]`, `[i]`, `[>]`) are unchanged.
- [ ] No non-logging behaviour is altered (engine, ledger, state, diff, layout all untouched).

## Implementation Plan / Notes
- Pure change in `snapshotter.go` around the current `fmt.Printf("Snapshot created: %s\n", event.ID)` line.
- Dominant-action precedence needs deciding when a snapshot mixes action kinds
  (e.g. a `MODIFY` and a `MOVE` in the same commit). Suggested approach: scan
  `event.Changes` and choose one label by a fixed precedence. Confirm the
  precedence (e.g. move > modify, or report based on which kind is most
  prevalent) during implementation.
- Decide how `CREATE` / `DELETE` / `IGNORED`-only snapshots are labelled (e.g.
  `(created)`, `(deleted)`, `(ignored)`), or whether the generic wording is kept
  for those; only `(modified)` and `(moved)` are explicitly requested today.
