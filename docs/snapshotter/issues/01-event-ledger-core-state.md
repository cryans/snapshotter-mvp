# Issue 01: Event Ledger & Core State

## Objective
Implement the `ledger.go` to append events to `.snapshots/.internal/events.jsonl` and write the human-readable blob history to `.snapshots/<path>/...`.

## Tasks
- [ ] Implement `EventAppender` (or similar) to write JSONL.
- [ ] Implement the mirrored tree blob writer for `CREATE`, `MODIFY`, `DELETE`, and `MOVE` handling `.moved`, `.deleted`, and `.moved_from` extensions.
- [ ] Ensure tests in `engine_test.go` start passing (or at least no longer skip for the ledger portion).
