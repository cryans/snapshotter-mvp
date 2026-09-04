# Issue 03: Rich CLI Commands and Robust Logging

## Objective
Extend `snapshotter.go` with explicit subcommand parsing (e.g., `snapshotter status`, `snapshotter commit "<msg>"`, and `snapshotter log`) to allow users to inspect changes before writing, supply custom commit messages, and view past snapshot ledger entries.

## Tasks
- [ ] Add subcommands using standard library `flag`:
  - `status`: Show tracked, untracked, modified, deleted, and moved files compared to the last projection *without* writing a new snapshot.
  - `commit "<message>"`: Create a new snapshot event with a custom user message (defaulting to "Snapshot").
  - `log`: Parse and print the append-only ledger `events.jsonl` in a clean, human-readable timeline.
- [ ] Ensure any error handling is output cleanly to `stderr` and exits with appropriate non-zero status codes.
