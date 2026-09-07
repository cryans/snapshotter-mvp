---
id: "08"
title: "CLI snapshot output should annotate each change line with a worded action label"
module: "snapshotter"
status: "completed"
branch: ""
created: "2026-09-04T13:00:00Z"
updated: "2026-09-07T11:49:00Z"
blocked_by: []
---

# Issue 08: CLI snapshot output should annotate each change line with a worded action label

> **Scoping note (2026-09-07):** This issue was delivered as per-change worded
> action labels on the snapshot output lines (`[~] (modified) path`, etc.), landed
> on `main` at `5922163`. The originally-proposed *header* parenthetical
> (`Snapshot created (modified): <ID>`) was **not** pursued; the per-line labels
> surfaced the action kinds instead and the header line stays generic. Issue body
> below was updated to match the delivered scope.

## Description
After a snapshot the CLI prints a header plus one line per change:

```
Snapshot created: 01M1P74NPYXMVN19ZRKW8ABKV8
  [~] src/main.go
```

Each per-change line shows only a compact glyph (`[+]`, `[~]`, `[-]`, `[i]`, `[>]`),
so a reader still has to recall what each glyph means. The output should instead
annotate each line with a worded action label so the snapshot's action kinds are
immediately legible:

```
Snapshot created: 01M1P74NPYXMVN19ZRKW8ABKV8
  [~] (modified) src/main.go
```

This is **logging/output wording only** — leave all other behaviour as is. No
engine, ledger, state, diff, or file-layout changes are part of this issue.

## Desired output
Each per-change line pairs its compact glyph with a lowercase worded action label:

```
Snapshot created: 01M1P74NPYXMVN19ZRKW8ABKV8
  [+] (created)   top-level.txt
  [~] (modified)  src/main.go
  [>] (moved)     hello.sh -> renamed.sh
```

The header line is unchanged (`Snapshot created: <ID>`).

## Acceptance Criteria
- [x] When a snapshot has changes, each per-change line reads `[<glyph>] (<action>) <path>` instead of the bare `[<glyph>] <path>`.
- [x] Modifications are labelled `(modified)`; moves are rendered `(moved) <old> -> <new>`.
- [x] CREATE / DELETE / IGNORED lines read `(created)` / `(deleted)` / `(ignored)`.
- [x] The header line still reads `Snapshot created: <ID>`.
- [x] No non-logging behaviour is altered (engine, ledger, state, diff, layout all untouched).

## Implementation Plan / Notes
- Pure change in `snapshotter.go`. Landed as commit `5922163`, which extracted the
  per-line rendering into `changeGlyph` / `describeChange` / `labelFor` helpers:
  - `changeGlyph(action)` returns the compact `[+]`/`[~]`/`[-]`/`[i]`/`[>]` marker.
  - `describeChange(change)` renders `(moved) old -> new` for moves, else
    `(<label>) <path>`.
  - `labelFor(action)` maps an action kind to its lowercase worded label.
- A dominant-action *header* label was considered (see original wording) but
  deliberately deferred/not chosen; per-line labels carry that information instead.
