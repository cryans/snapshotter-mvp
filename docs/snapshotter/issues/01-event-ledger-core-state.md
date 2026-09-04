---
id: "01"
title: "Event Ledger Core State"
module: "snapshotter"
status: "completed"
branch: ""
created: "2026-09-04T12:00:00Z"
updated: "2026-09-04T12:00:00Z"
blocked_by: []
---

# Issue 01: Event Ledger & Core State

## Description
Implement the `ledger.go` to append events to `.snapshots/.internal/events.jsonl` and write the human-readable blob history to `.snapshots/<path>/...`.

## Acceptance Criteria
- [x] Implement `EventAppender` (or similar) to write JSONL.
- [x] Implement the mirrored tree blob writer for `CREATE`, `MODIFY`, `DELETE`, and `MOVE` handling `.moved`, `.deleted`, and `.moved_from` extensions.
- [x] Ensure tests in `engine_test.go` start passing (or at least no longer skip for the ledger portion).
