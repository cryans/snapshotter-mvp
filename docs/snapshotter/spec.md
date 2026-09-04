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
