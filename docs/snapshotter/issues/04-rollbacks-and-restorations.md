---
id: "04"
title: "Snapshot Rollbacks and Content Restorations"
module: "snapshotter"
status: "ready-to-implement"
branch: ""
created: "2026-09-04T12:00:00Z"
updated: "2026-09-04T13:29:00Z"
blocked_by: []
---

# Issue 04: Snapshot Rollbacks and Content Restorations

## Description
Implement a mechanism to restore files to their exact state from a historical commit ID, using either the authoritative ledger metadata or by copying the stored blobs back out of the mirrored tree.

## Acceptance Criteria
- [ ] Implement `Restore(commitID string, targetPath string)` in the core engine.
- [ ] Create a `snapshotter restore <commit-id> [path]` CLI command.
- [ ] Safely write files back to the working directory, ensuring we do not overwrite untracked files without warning.
- [ ] Add rigorous unit tests simulating full restorations of deleted and modified files.

## Ordering & Rationale (roadmap)
- **Sequencing:** land `05` (Action Lineage) before this issue. Restore resolution consumes
  the shared change-event model `05` extends; writing it after `05` allows lineage-aware
  restore of content that lived at moved paths. See `docs/snapshotter/roadmap.md`.
- This is an **ordering/priority** relationship, *not* a hard dependency: restore-by-commit
  can copy blobs back from the mirrored tree using commit ULIDs + timestamps today.
