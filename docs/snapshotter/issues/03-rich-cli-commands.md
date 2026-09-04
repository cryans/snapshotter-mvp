---
id: "03"
title: "Interactive TUI File History Viewer"
module: "snapshotter"
status: "proposed"
branch: ""
created: "2026-09-04T12:00:00Z"
updated: "2026-09-04T13:00:00Z"
blocked_by: ["01", "02"]
---

# Issue 03: Interactive TUI File History Viewer

## Description
When a user targets a file with `snapshotter <filename>`, launch an interactive Terminal User Interface (TUI) displaying that file's complete chronological history.

## TUI Layout & Interactions
- **Ordering**: Timestamps must be shown in descending order (newest first).
- **Labels**:
  - The current state must be marked with `(current)`.
  - Deleted entries must be marked with `(deleted)`.
  - Moved entries must be marked with `(moved)`.
- **Navigation**:
  - Pressing `Enter` on a `(moved)` entry should automatically navigate the TUI to the history of the new destination path (as if the user had executed `snapshotter <new_filename>`).

## Acceptance Criteria
- [ ] Parse cli arguments to detect the targeted filename.
- [ ] Launch a lightweight, responsive TUI in the terminal.
- [ ] Display historical entries ordered newest-first with accurate `(current)`, `(deleted)`, and `(moved)` annotations.
- [ ] Wire up `Enter` on a `(moved)` entry to transition the view to the history of the new file location.
- [ ] Provide clean error handling if the specified file does not exist in both active workspace and ledger.
