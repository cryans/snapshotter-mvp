---
id: "07"
title: "Add end-to-end engine integration tests"
module: "snapshotter"
status: "proposed"
branch: ""
created: "2026-09-04T12:27:33Z"
updated: "2026-09-04T12:27:33Z"
blocked_by: []
---

# Issue 07: Add end-to-end engine integration tests

## Description
Add a suite of integration tests that exercise the engine end-to-end,
mirroring the manual walk-through performed against `/tmp/snap-demo` during
issue 02 (first snapshot, `.gitignore` introduction, negation + nested rules,
move detection, and path/format inspection). Each test must fully set up and
tear down its own temporary file tree, and should avoid depending on real,
shared workspace state.

## Acceptance Criteria
- [ ] Write the tests in Go using the existing test helpers, each as a self-contained test that creates its own temporary tree and cleans it up afterward.
- [ ] Scenario: initial snapshot of a freshly created tree produces expected `CREATE` events and the mirrored `.snapshots/` layout.
- [ ] Scenario: hard-coded exclusions (`.snapshots/`, `.git/`, including nested `.git`) are never snapshotted.
- [ ] Scenario: adding a `.gitignore` re-runs the snapshot and reflects newly-ignored paths without tracking them (aligning with issue 06's intended `IGNORED` behavior once implemented).
- [ ] Scenario: `.gitignore` negation (`!pattern`) re-includes a matched path.
- [ ] Scenario: nested `.gitignore` rules apply only within their own directory scope.
- [ ] Scenario: content move is detected as a single `MOVE` with the correct old/new paths.
- [ ] Scenario: every ledger/change path uses forward slashes regardless of host OS separator.
- [ ] Where feasible, minimize real-filesystem coupling (e.g. drive file trees through a thin abstraction or throwaway temp trees) while keeping the tests truly end-to-end.

## Implementation Plan / Notes
- Follows the manual demo script from the issue 02 integration walk-through.
- `t.TempDir()` provides automatic per-test setup/teardown; note the walker/diff inherently read real bytes off disk, so "without hitting the filesystem" is an aspiration (abstract the filesystem for in-memory runs) rather than an absolute, unless we first introduce a filesystem interface.
- Likely a new `internal/store/engine_integration_test.go`.
- Coordinate with issue 06 so the ignore-behavior assertion lands on the final representation rather than the current `DELETE` interim behavior.
