# AI Engineering Context (AGENTS.md)

This document provides essential context, architectural rules, and layout constraints for AI agents cooperating on `snapshotter`. Read this before proposing or making changes.

## Project Purpose
`snapshotter` is a local-first, deterministic file snapshotting engine designed to run entirely in user-space with zero external dependencies (standard-library Go only, Go 1.23+). 

Instead of daemon-based file-system watching (e.g., `fsnotify`), it relies on a clean, unidirectional data flow:
`Physical Disk Walk -> Pure Diff Engine -> CommitEvent -> Append Ledger & Blobs -> Update Projection`

---

## Storage & Metadata Layout

All tool metadata and history live inside a directory named `.snapshots/` at the workspace root:

```
├── .snapshots/
│   ├── .internal/
│   │   └── events.jsonl         <--- Source of Truth (Append-only JSON Lines log)
│   ├── hello.sh/
│   │   ├── 2026-09-03T12-02-15.510.sh
│   │   ├── 2026-09-03T12-02-26.853.sh
│   │   └── 2026-09-03T12-02-30.120.sh.deleted
│   └── src/
│       └── main.go/
│           ├── 2026-09-03T12-03-07.099.go.moved_from  <--- (Contains original path)
│           └── 2026-09-03T12-03-07.099.go             <--- Actual content
├── hello.sh
└── src/
    └── main.go
```

### Historical Storage Rules
- **Creates & Modifies**: Actual file contents are saved at `.snapshots/<path>/<timestamp>.<ext>`.
- **Deletes**: Recorded as an empty marker at `.snapshots/<path>/<timestamp>.<ext>.deleted`.
- **Moves**: 
  - At the old location: `.snapshots/<old_path>/<timestamp>.<ext>.moved` (contains the new path text).
  - At the new location: `.snapshots/<new_path>/<timestamp>.<ext>.moved_from` (contains the old path text), accompanied by the copied file contents at `.snapshots/<new_path>/<timestamp>.<ext>`.

---

## Directory Exclusions (Strict Rules)
When scanning the working directory, the walker **must** ignore the following directories and files to avoid infinite loops, self-snapshots, or committing massive localized caches:

1. **Internal Tool State**: `.snapshots/`
2. **Local Workspace Cache**: `.go-cache/` and `.go-mod-cache/` (used by the sandbox runner)
3. **Spec and Issue Tracking**: `.scratch/`
4. **VCS Directories**: `.git/`
5. **Compiled Executables**: Any compiled binary (e.g., `snapshotter_bin`)

---

## Phase 1 Engine Components (`internal/store/`)
1. `types.go`: Domain models representing the core states (`CommitEvent`, `Change`, `FileState`, `Tombstone`, `Projection`).
2. `id.go`: Custom 26-character Crockford Base32 stateless ULID generator (zero-dependency).
3. `ledger.go`: Responsible for appending structured events to `.snapshots/.internal/events.jsonl` and writing the mirrored tree storage layout.
4. `state.go`: Pure in-memory Projection reducer replaying the append-only ledger to build active file lists and tombstones.
5. `engine.go` (integrates `diff.go` behaviors): Walks the directory, computes difference records, identifies moved files by content hashes, and initiates snapshot commits.
