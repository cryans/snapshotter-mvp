# Issue 02: Refine Path Normalization and Ignoring Rules

## Objective
Ensure that the directory walker consistently normalizes path separators across platforms (converting all sub-paths to forward slashes `ToSlash`) and robustly ignores `.snapshots`, `.scratch`, and build artifacts (like `snapshotter_bin`) to prevent self-snapshotting.

## Tasks
- [ ] Explicitly ignore any directory or file starting with `.` (like `.snapshots`, `.scratch`, `.git`).
- [ ] Exclude compiled binaries (such as `snapshotter_bin`) and localized test cache folders (`.go-cache`).
- [ ] Ensure all relative paths generated during physical walk are explicitly converted to forward slashes so the log remains completely platform-independent (Windows/macOS/Linux).
- [ ] Add comprehensive test coverage in `engine_test.go` confirming exclusion rules.
