---
id: "02"
title: "Refine Path Normalization and Ignoring Rules"
module: "snapshotter"
status: "completed"
branch: "feature/02-path-normalization-and-ignoring"
created: "2026-09-04T12:00:00Z"
updated: "2026-09-04T14:30:00Z"
blocked_by: []
---

# Issue 02: Refine Path Normalization and Ignoring Rules

## Description
Ensure that the directory walker consistently normalizes path separators across platforms (converting all sub-paths to forward slashes `ToSlash`) and robustly handles exclusions. The `.gitignore` parser is integrated to drive high-fidelity ignoring behavior.

Design decisions confirmed during implementation:
- Integrate the standard community `.gitignore` parser (`github.com/sabhiram/go-gitignore`) to parse patterns dynamically, including root + nested `.gitignore` files with git-style semantics.
- Only the tool's own state directory (`.snapshots`) is hard-coded and unconditionally excluded. Everything else follows `.gitignore` rules; dot-entries or compiled binaries are **not** blanket auto-ignored (e.g. `.git`, `.scratch`, `.go-cache`, `snapshotter_bin` must be listed in a `.gitignore`).
- The single external dependency is documented as an exception in `AGENTS.md` / `spec.md`, which otherwise mandate standard-library Go only.

## Acceptance Criteria
- [x] Integrate the `github.com/sabhiram/go-gitignore` library to parse `.gitignore` patterns dynamically, honoring root and nested `.gitignore` files with git-style scoping (see `internal/store/walker.go`).
- [x] Always ignore the tool's own state directory (`.snapshots`), regardless of `.gitignore` rules.
- [x] Follow `.gitignore` rules for other exclusions (e.g. `.git`, `.scratch`, `.go-cache`, `snapshotter_bin`) rather than blanket-ignoring every dot-entry or binary.
- [x] Ensure all relative paths generated during physical walk are explicitly converted to forward slashes so the log remains completely platform-independent (Windows/macOS/Linux).
- [x] Add comprehensive test coverage in `engine_test.go` confirming hard-coded exclusions, root/nested `.gitignore` matching, negation, directory-only patterns, and forward-slash normalization.

## Implementation Notes
- `internal/store/walker.go` introduces a `walker` that descends from the scan root, loading each directory's `.gitignore` as a frame scoped to that directory (matching git's per-directory rule anchoring), and prunes excluded directories so a deeper rule cannot re-include inside an excluded parent.
- `engine.go` diff now consumes the walker's tracked files instead of inlining skip logic, so exclusion is centralized and testable.
- `go.mod` / `go.sum` updated to add `github.com/sabhiram/go-gitignore` as a direct dependency.

