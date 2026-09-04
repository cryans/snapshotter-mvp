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
5. **Tech Stack**: Standard library Go 1.23 only for `internal/store`.
