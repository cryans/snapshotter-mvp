# AI Engineering Context (AGENTS.md)

This document provides essential context, architectural rules, and layout constraints for AI agents cooperating on `snapshotter`. Read this before proposing or making changes.

## Repository Standards & Issue Workflow
For documentation layout, file-based issue tracking, and issue templates, refer to `docs/standards.md`.

## Git Hygiene
Never run `git add`, `git commit`, or `git push` unless the user has explicitly asked you to do so. Make changes to the working tree freely, but leave version-control staging, committing, and pushing to the user unless directly requested.

## Build & Test
Prefer the provided `Makefile` over invoking Go directly for common tasks. Use `make build`, `make run`, and `make test` (and `make clean`) whenever they fit; fall back to raw `go`/`go test` only when a task is not covered by a target.

## Project Purpose
`snapshotter` is a local-first, deterministic file snapshotting engine designed to run almost entirely in user-space. The engine code in `internal/store` is standard-library Go only, with **one documented dependency exception**: `github.com/sabhiram/go-gitignore`, used to parse `.gitignore` patterns in `internal/store/walker.go`. Keep any further dependencies out of the engine.

The **interactive history TUI** (root `main` package, `history_tui.go`) is the repo's second documented dependency exception: `github.com/charmbracelet/bubbletea` (with its `lipgloss` companion) drives the terminal UI that `snapshotter <filename>` launches. This UI code deliberately sits outside `internal/store`, so the engine's pure-stdlib invariant is preserved.

The module's `go` directive is 1.24 (raised from 1.23 by bubbletea's transitive `golang.org/x/sys`, which requires Go >= 1.24); the stated floor remains "Go 1.23+" and the toolchain must be at least 1.24 to build with the TUI.

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
- **Ignored** (file still on disk but newly excluded by an ignore rule): Recorded as an empty marker at `.snapshots/<path>/<timestamp>.<ext>.ignored`. Historical snapshots of that path are left untouched; only live tracking stops (issue 06).
- **Moves**: 
  - At the old location: `.snapshots/<old_path>/<timestamp>.<ext>.moved` (contains the new path text).
  - At the new location: `.snapshots/<new_path>/<timestamp>.<ext>.moved_from` (contains the old path text), accompanied by the copied file contents at `.snapshots/<new_path>/<timestamp>.<ext>`.

---

## Directory Exclusions
To avoid infinite loops, self-snapshots, or committing massive localized caches, the walker enforces a small hard-coded default plus standard `.gitignore` semantics:

1. **Hard-coded (always excluded, non-overridable)**: the tool's own state dir `.snapshots/` and VCS metadata `.git/`. The walker never snapshots its own ledger or a repository's plumbing, regardless of `.gitignore`. (`.git` is excluded because git itself never consults `.gitignore` for its internal directories — not because a pattern matches it.)
2. **Everything else is `.gitignore`-driven** (`internal/store/walker.go` via `github.com/sabhiram/go-gitignore`, root + nested files, git-style). Blanket auto-ignoring of every dot-entry or binary is **not** done. To exclude the sandbox caches, spec/issue tracking, or compiled binaries during real runs, users list them in a `.gitignore`:
   - Local workspace cache: `.go-cache/`, `.go-mod-cache/`
   - Spec and issue tracking: `.scratch/`
   - Compiled executables, e.g. `snapshotter_bin`

---

## Phase 1 Engine Components (`internal/store/`)
1. `types.go`: Domain models representing the core states (`CommitEvent`, `Change`, `FileState`, `Tombstone`, `Projection`).
2. `id.go`: Custom 26-character Crockford Base32 stateless ULID generator (zero-dependency).
3. `ledger.go`: Responsible for appending structured events to `.snapshots/.internal/events.jsonl` and writing the mirrored tree storage layout.
4. `state.go`: Pure in-memory Projection reducer replaying the append-only ledger to build active file lists and tombstones.
5. `engine.go` (integrates `diff.go` behaviors): Walks the directory, computes difference records, identifies moved files by content hashes, and initiates snapshot commits.
