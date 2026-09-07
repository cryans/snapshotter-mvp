---
id: "09"
title: "Engine diff can miss equal-length modifications within filesystem timestamp granularity"
module: "snapshotter"
status: "completed"
branch: "feature/09-engine-diff-modtime-short-circuit"
created: "2026-09-04T14:13:43Z"
updated: "2026-09-07T11:06:00Z"
blocked_by: []
---

# Issue 09: Engine diff can miss equal-length modifications within filesystem timestamp granularity

## Description

In `internal/store/engine.go`, `Engine.diff` skips re-hashing an already-tracked file
when both its recorded size and modification time match the file on disk:

```go
// Optimization: if size and modtime exactly match, skip hashing
if exists && state.Size == df.info.Size() && state.ModTime.Equal(df.info.ModTime()) {
    continue
}
```

This short-circuit is correct only when an unchanged file keeps both properties. It
becomes incorrect when a file is genuinely modified in a way that preserves the **size**
(new content of identical length) **and** the modification time lands within the
filesystem's timestamp granularity of the previously-recorded `mod_time` (so
`ModTime.Equal` reports them equal). In that case the modification is silently dropped:
no `MODIFY` change is emitted, no blob is mirrored, and the ledger never learns the file
changed.

This surfaced while writing the Issue 04 restore integration tests, where two distinct
bodies of equal length written in rapid succession were not detected as a change. The
tests were worked around by using differing-length contents; the underlying engine bug is
still present.

The size+modtime equality is a heuristic, not a guarantee. Content can change while the
size stays identical, and coarse timestamp granularity (e.g. some filesystems / network
shares / copy tools that stamp mtimes) makes the mtime half of the check unreliable for
*recently* written files. `mod_time` equality is also not proof of byte-identity.

## Acceptance Criteria
- [x] The diff produces a `MODIFY` change for an already-tracked file whose content
      changes but whose byte length is unchanged, regardless of how close in time the two
      writes are.
- [x] Unchanged files are still skipped (the fast path must not regress into re-hashing
      every file on every snapshot).
- [x] The change is covered by a test that reproduces an equal-length, same-timestamp
      modification.

## Resolution

Adopted the **grace-period fast path** (see **DD-02** in `spec.md`): the size+modtime
short-circuit in `internal/store/engine.go` is only trusted once the recorded `mod_time`
is at least `modTimeGrace` (1s) old relative to the snapshot time. Files written within
the grace window fall through to hashing, so an equal-length write that the filesystem
reports with the same mtime is detected. `Snapshot` was split into an internal
`snapshotAt(dir, now)` so tests drive the snapshot time deterministically instead of
relying on the wall clock.

Tests added in `internal/store/engine_test.go`:
- `TestEngine_EqualLengthSameMtime_Recent_Detected` — deterministic (injected `now`)
  reproduction; verified to fail against the pre-fix fast path.
- `TestEngine_EqualLengthSameMtime_PublicAPI` — end-to-end through `Snapshot()`, including
  a fresh-engine reload from the ledger.
- `TestEngine_UnchangedColdFile_FastPathStillSkips` — unchanged cold file still
  short-circuits (AC2 guard).

## Implementation Plan / Notes
- The `mod_time` equal + `size` equal fast path is fundamentally unsafe as the sole signal
  for "unchanged". It can only be trusted when the recorded `mod_time` is *older* than a
  guaranteed-granularity boundary (i.e. sufficiently in the past that an equal mtime can no
  longer be a coincidental recent write).
- Candidate approaches:
  - Drop the mtime half and rely on size + always-hash (safest, but re-hashes all unchanged
    same-size files each snapshot, losing the fast path for large trees).
  - Only trust `size == && modTime.Equal` when `modTime` is at least one granularity
    quantum old (e.g. older than a filesystem-typical resolution, ~1s), else fall through
    to hashing. Keeps the fast path for cold files while catching recent writes.
  - Store and compare the blob hash as the authoritative signal and demote size/mtime to a
    pure optimization with an explicit correctness fallback.
- Confirm which filesystems the repo targets and whether a fixed grace period is acceptable,
  or whether hashing is preferred for correctness.
