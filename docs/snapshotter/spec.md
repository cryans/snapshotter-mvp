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
6. **Tech Stack**: Standard library Go 1.23 for `internal/store`, with the single documented exception of `github.com/sabhiram/go-gitignore` for parsing `.gitignore` rules (see `AGENTS.md`).
7. **Regular Files Only / Empty Directories**: Only regular files are ever snapshotted. Directories themselves are never tracked, so creating or removing an empty directory produces no events — only files inside it do. Consequently a commit is recorded only when at least one file-level change exists.
