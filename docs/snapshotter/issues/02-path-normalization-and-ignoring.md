---
id: "02"
title: "Refine Path Normalization and Ignoring Rules"
module: "snapshotter"
status: "ready-to-implement"
branch: ""
created: "2026-09-04T12:00:00Z"
updated: "2026-09-04T13:00:00Z"
blocked_by: []
---

# Issue 02: Refine Path Normalization and Ignoring Rules

## Description
Ensure that the directory walker consistently normalizes path separators across platforms (converting all sub-paths to forward slashes `ToSlash`) and robustly ignores `.snapshots`, `.scratch`, and build artifacts. To achieve high-fidelity ignoring behavior matching user expectations, we will integrate the standard community `.gitignore` parser.

## Acceptance Criteria
- [ ] Integrate the `github.com/sabhiram/go-gitignore` library to parse `.gitignore` patterns dynamically.
- [ ] Explicitly ignore any directory or file starting with `.` (like `.snapshots`, `.scratch`, `.git`).
- [ ] Exclude compiled binaries (such as `snapshotter_bin`) and localized test cache folders (`.go-cache`).
- [ ] Ensure all relative paths generated during physical walk are explicitly converted to forward slashes so the log remains completely platform-independent (Windows/macOS/Linux).
- [ ] Add comprehensive test coverage in `engine_test.go` confirming exclusion and `.gitignore` matching rules.
