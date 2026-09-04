---
id: "07"
title: "Add end-to-end engine integration tests"
module: "snapshotter"
status: "completed"
branch: "feature/07-engine-integration-tests"
created: "2026-09-04T12:27:33Z"
updated: "2026-09-04T13:12:58Z"
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
- [x] Write the tests in Go using the existing test helpers, each as a self-contained test that creates its own temporary tree and cleans it up afterward.
- [x] Scenario: initial snapshot of a freshly created tree produces expected `CREATE` events and the mirrored `.snapshots/` layout.
- [x] Scenario: hard-coded exclusions (`.snapshots/`, `.git/`, including nested `.git`) are never snapshotted.
- [x] Scenario: adding a `.gitignore` re-runs the snapshot and reflects newly-ignored paths without tracking them, asserting the `IGNORED` action and `.ignored` markers from issue 06 (now implemented), rather than `DELETE`/`.deleted`.
- [x] Scenario: `.gitignore` negation (`!pattern`) re-includes a matched path.
- [x] Scenario: nested `.gitignore` rules apply only within their own directory scope.
- [x] Scenario: content move is detected as a single `MOVE` with the correct old/new paths.
- [x] Scenario: every ledger/change path uses forward slashes regardless of host OS separator.
- [x] Where feasible, minimize real-filesystem coupling (e.g. drive file trees through a thin abstraction or throwaway temp trees) while keeping the tests truly end-to-end.

## Implementation Plan / Notes
- Follows the manual demo script from the issue 02 integration walk-through.
- `t.TempDir()` provides automatic per-test setup/teardown; note the walker/diff inherently read real bytes off disk, so "without hitting the filesystem" is an aspiration (abstract the filesystem for in-memory runs) rather than an absolute, unless we first introduce a filesystem interface.
- Likely a new `internal/store/engine_integration_test.go`.
- Issue 06 is now implemented: newly-ignored files emit `ActionIgnored` and write `.ignored` markers. The integration test for the ignore scenario should assert against this final representation (and the full ignore lifecycle in issue 06 — IGNORED on rule introduction, then `CREATE` when the rule is removed). The scenario can be exercised via the `Engine.Snapshot` flow already covered by unit tests in `engine_test.go`, but at end-to-end integration granularity.

### Reference: manual walk-through script (source of the scenarios)

This is the canonical end-to-end sequence the integration tests should mirror.
Run against a scratch tree under `/tmp` (never the repo workspace). The
expected per-step change set is annotated after each `snapshotter` invocation;
assert on the set of `(action, path)` pairs — exact ordering may vary since the
engine emits from map iteration.

```bash
SNAP=/home/duben/projects/clean-slate-snapshotter/bin/snapshotter

# --- fresh tree: seed some files ---
mkdir -p /tmp/snap-demo/sub/deep
cd /tmp/snap-demo

echo "hello world" > readme.md
echo "secret data" > secret.log
echo "cachey"      > sub/cache.tmp
echo "nested"      > sub/deep/note.txt
mkdir build
echo "binary-ish"  > build/out.bin

# --- snapshot 1: everything tracked as CREATE ---
$SNAP
# expected: CREATE readme.md, CREATE secret.log, CREATE sub/cache.tmp,
#           CREATE sub/deep/note.txt, CREATE build/out.bin

# --- snapshot 2: no changes -> "No changes detected." ---
$SNAP
# expected: nil event / "No changes detected."

# --- modify a tracked file -> MODIFY ---
echo "hello world v2" > readme.md
$SNAP
# expected: MODIFY readme.md

# --- move a file -> MOVE (old -> new) ---
mv sub/cache.tmp sub/cache2.tmp
$SNAP
# expected: MOVE sub/cache.tmp -> sub/cache2.tmp (single event; content hash
#           matches the tracked-but-missing source, so no CREATE). After this,
#           the file is tracked at sub/cache2.tmp.
# history:  .snapshots/sub/cache.tmp/ keeps its original content snapshot and a
#           <ts>.tmp.moved marker; .snapshots/sub/cache2.tmp/ gets the copied
#           content + a <ts>.tmp.moved_from marker.

# --- add an ignore rule -> newly-ignored files flip to [i] IGNORED (issue 06) ---
printf '*.log\n*.tmp\nbuild/\n' > .gitignore
$SNAP
# expected: CREATE .gitignore,
#           IGNORED secret.log, IGNORED sub/cache2.tmp, IGNORED build/out.bin
#           (each still exists on disk but is now excluded; must NOT be DELETE)

# --- inspect: .ignored markers present, history preserved ---
ls -1 .snapshots/secret.log/            # original <ts>.log content snapshot remains + a <ts>.log.ignored marker
ls -1 .snapshots/sub/cache2.tmp/        # copied content + <ts>.tmp.moved_from marker + <ts>.tmp.ignored marker
# NOTE: sub/cache.tmp is gone as a live path (it was moved); inspect under
#       cache2.tmp for the .ignored marker.

# --- remove the ignore rule -> files re-tracked as CREATE ---
rm .gitignore
$SNAP
# expected: DELETE .gitignore,
#           CREATE secret.log, CREATE sub/cache2.tmp, CREATE build/out.bin
#           (previously-ignored files reappear as CREATE, not MODIFY/MOVE)

# --- rm / rmdir: true deletions still emit [-] DELETE ---
rm sub/deep/note.txt
rmdir sub/deep            # removes the now-empty dir (not snapshotted)
rm build/out.bin
rmdir build
$SNAP
# expected: DELETE sub/deep/note.txt, DELETE build/out.bin
#           (rmdir on directories emits nothing — dirs aren't snapshotted)

# --- look at the append-only ledger ---
cat .snapshots/.internal/events.jsonl   # one JSON line per commit
```

#### Notes / gotchas to encode as test assertions
- Empty directories are never snapshotted, so `mkdir`/`rmdir` alone produce no
  events; only files inside them do.
- `mv` of a tracked file yields a single `MOVE` (source matched by content hash),
  not a `DELETE` + `CREATE`.
- Adding an ignore rule over still-present files produces `IGNORED` (issue 06),
  while a genuine `rm` still produces `DELETE`.
- Removing the ignore rule re-tracks as `CREATE` without duplicating history.
- Start each test from a fresh scratch tree and tear it down; never point the
  tool at shared/real workspace state.

### Implementation summary
- Added `internal/store/engine_integration_test.go` (package `store`), one test
  function per acceptance-criteria scenario. Each test builds its engine with
  `NewEngine(root, root/.snapshots)` so the state mirror lives *inside* the
  scanned tree — the same wiring the CLI uses — which also exercises the
  walker's hard-coded self-exclusion on a follow-up snapshot.
- `TestIntegration_ContentMoveDetected` confirms the single `MOVE` (keyed by its
  destination path) plus the `.moved` / `.moved_from` markers in the mirror.
- The ignore scenario asserts the full issue-06 lifecycle at integration
  granularity: `IGNORED` on rule introduction (with `.ignored` markers and
  preserved history, never `.deleted`), then re-tracking as `CREATE` when the
  rule is removed.
- The "empty directories are never snapshotted" invariant surfaced during this
  walk-through is now documented as Architectural Invariant #7 in `spec.md`.
