# Snapshotter Specification

## Overview
A local-first, deterministic file snapshotting engine.

## Architectural Invariants
1. **Source of Truth**: Single append-only ledger at `.snapshots/.internal/events.jsonl`.
2. **Storage Layout**: Mirrored tree approach for easy CLI/bash browsing (`grep`, `find`).
   - Creates/Updates: `.snapshots/<path>/<timestamp>.<ext>`
   - Deletes: `.snapshots/<path>/<timestamp>.<ext>.deleted`
   - Moves (Old Location): `.snapshots/<old_path>/<timestamp>.<ext>.moved` (contains new path)
   - Moves (New Location): `.snapshots/<new_path>/<timestamp>.<ext>.moved_from` (contains old path) followed by actual content file.
3. **Identifiers**: Statelessly generated 26-character Crockford Base32 ULIDs.
4. **Move Detection**: Purely in-memory correlation of identical hashes between missing active files and new untracked files.
5. **Exclusions**: `.snapshots/` (the tool's own state) and `.git/` (VCS metadata) are always hard-excluded from any walk. All other exclusions (e.g. `.scratch/`, `.go-cache/`, compiled binaries) are governed by the standard `.gitignore` parser — users list them in a `.gitignore` rather than relying on blanket dot-file or binary-name auto-ignoring.
6. **Tech Stack**: Standard library Go for `internal/store`, with the single documented exception of `github.com/sabhiram/go-gitignore` for parsing `.gitignore` rules (see `AGENTS.md`). The interactive history TUI that `snapshotter <filename>` launches lives in the root `main` package and uses the `charmbracelet/bubbletea` TUI framework (with `lipgloss`), a second documented exception confined to that UI code — `internal/store` itself stays pure standard library. The module's `go` directive is 1.24 (see `AGENTS.md`).
7. **Regular Files Only / Empty Directories**: Only regular files are ever snapshotted. Directories themselves are never tracked, so creating or removing an empty directory produces no events — only files inside it do. Consequently a commit is recorded only when at least one file-level change exists.

## Design Decisions

Design decisions are recorded here (ADR-style) when they are made and confirmed,
so a future session does not re-litigate a settled question. Each entry states the
context, the decision, and the rationale/tradeoffs.

### DD-01: A file copy is recorded as a CREATE, not a COPY (2026-09-07)

**Context.** `cp a b` (both files then live/tracked) is reported by the engine as
a `CREATE` of `b`, with an empty `PreviousID`. The question arose whether `b`
should instead be labelled a `COPY`, whether `PreviousID` should be populated from
the source `a`'s lineage, and what git does in the same situation.

**Decision.** A byte-identical new file whose source is still present is recorded
as a plain `CREATE` — a fresh lineage root with an empty `PreviousID`. Copy is
**not** promoted to a first-class action type, and the engine does not attempt to
link `b` to `a` at write time. This matches git: git never records a copy/rename in
a commit (a commit is just a content-addressed snapshot of the whole tree); copy
detection exists only as an **opt-in, read-time display heuristic** (`git diff -C` /
`--find-copies` / `--find-copies-harder`), off by default, and it never persists a
relationship. snapshotter intentionally mirrors that default.

**Rationale / tradeoffs.**
- **Intent is not observable from a disk walk.** A byte-identical new file could be
  a deliberate `cp`, a hardlink copy, `cat a > b`, or an unrelated file that merely
  hashes the same. Only a filesystem-level signal (which the walk does not read)
  could distinguish copy intent; content equality is not proof of it.
- **A copy is a new logical path.** `PreviousID` links the action that produced the
  *previous version of this logical file* (single-parent lineage: CREATE→MODIFY→
  DELETE). `b` has no prior version, and auto-linking it to `a` would create a
  **branching** lineage (a fork); a later modify to `a` or `b` could then not
  disambiguate which branch is meant. Git avoids storing renames/copies partly for
  this reason — a file is not a stable identity in the model.
- **Copy is not MOVE, and the engine already keeps them apart.** `MOVE` fires only
  when a hash matches a *physically missing* tracked file. Because `a` is still
  present it is never a move candidate, so `cp` can never be misreported as a
  MOVE/DELETE. Copying onto an already-deleted-origin *would* surface as a MOVE,
  which is correct (the original is gone).

**Future option (kept off the critical path).** If copy *reporting* is ever wanted,
do it like git: a read-time heuristic that annotates a new `CREATE` whose hash equals
a still-live file's hash as a "possible copy of `<src>`" — presentation only, no
schema/ledger/reducer change, and no branching lineage. Do not add an `ActionCopy`
to the engine unless a concrete need for persisting copy identity appears.

### DD-02: The "unchanged" fast path is gated on the recorded mtime's age (2026-09-07)

**Context.** Issue 09: `Engine.diff` skipped re-hashing an already-tracked file when
its recorded `size` and `mod_time` both matched the file on disk. That short-circuit
silently dropped a genuine `MODIFY` when a file was rewritten at *equal length* and the
second write's mtime fell within the filesystem's timestamp granularity of the recorded
one (so `ModTime.Equal` reported them equal).

**Decision.** Keep size+modtime as the fast-path signal, but only trust it once the
recorded `mod_time` is **older than a grace period** relative to the snapshot time. A
tracked file whose size and modtime match the record is skipped only when
`now - recorded mod_time >= modTimeGrace` (`modTimeGrace = 1s`, a named constant in
`internal/store/engine.go`); otherwise the file is re-hashed. Files written within the
last grace second are re-hashed so an equal-length recent write cannot be masked by an
equal mtime; stable/cold files keep the fast path.

**Rationale / tradeoffs.**
- **Correctness model.** Filesystems update mtime on every write, truncating only within
  one timestamp quantum. So once the recorded mtime is ≥ one quantum in the past, an
  identical on-disk mtime is trustworthy evidence that no re-write has occurred; an
  equal-length write within that window could still be masked, hence the fall-through.
  The heuristic does **not** defeat deliberate mtime-stamping by copy/archive tools that
  force an old mtime onto new equal-length content — no stat-based snapshotter can without
  hashing.
- **Fast path preserved (AC2).** Re-hashing is confined to files touched within the last
  ~1s. It does not regress into re-hashing every file on every snapshot, unlike the
  always-hash alternative.
- **Why not always-hash.** The "always hash tracked same-size files" option is maximally
  tamper-resistant but rehashes every unchanged file each snapshot — explicitly rejected
  by issue 09 AC2.
- **Fixed grace, not per-filesystem.** A single 1s grace covers typical coarse granularity
  (FAT, many network shares) without plumbing per-filesystem resolution. Bumped via the
  `modTimeGrace` constant if ever needed.

## Interactive File History Viewer (issue 03)

`snapshotter <filename>` opens a small interactive terminal viewer showing that
path's chronological history, **newest first**, reconstructed from the append-only
ledger. Rows carry `(current)` (newest entry while the file is still tracked),
`(deleted)`, `(moved)`, and `(ignored)` annotations.

### Enter semantics (clarified)
Pressing **Enter** on a highlighted row does one of these, depending on the row:

- **`(moved)` row → follow.** The view navigates to the history of the move's new
  destination (navigation only; nothing is written).
- **Historical content version (created / modified / moved-in) that is *not* the
  `(current)` row → restore.** The version's content is written back to the file
  whose name is currently displayed. Any divergent live content at that name is
  first preserved under `.snapshots/.internal/conflicts/` (the standard restore
  conflict backup). The tool then **auto-snapshots the workspace**, so the restored
  content is recorded as a new history entry that becomes the new `(current)` row;
  the view refreshes immediately.
- **`(current)` row → no-op.** A message notes the file is already at the current
  version; no snapshot is created.
- **`(deleted)` / `(ignored)` rows → no-op.** These are informational only.

Notes on the model:
- The ledger is append-only, so restoring a past version legitimately appends a new
  entry whose content is byte-identical to that earlier version — a distinct commit,
  not a rewrite. This is intended.
- The auto-snapshot triggered by a restore is a normal **whole-workspace** snapshot:
  any other pending changes present in the working tree at that moment are recorded
  in the same commit.
- Restores always target the currently displayed file name; following a `(moved)`
  row switches the displayed name to the destination first.
