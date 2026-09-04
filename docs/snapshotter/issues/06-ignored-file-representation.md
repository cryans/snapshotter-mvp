---
id: "06"
title: "Represent newly-ignored files as IGNORED, not DELETE"
module: "snapshotter"
status: "proposed"
branch: ""
created: "2026-09-04T12:27:33Z"
updated: "2026-09-04T12:27:33Z"
blocked_by: []
---

# Issue 06: Represent newly-ignored files as IGNORED, not DELETE

## Description
Today, when a file that is currently tracked in the projection becomes newly
ignored (e.g. the user adds a `.gitignore` rule matching it), the walker no
longer returns it, so the diff engine emits an `ActionDelete` for it and the
ledger writes a `.deleted` marker. That conflates "the file was physically
removed" with "the file is still there but is now excluded by an ignore rule."

We want a distinct representation so that *old history remains*, but the file
is simply **no longer tracked** from this point on — surfaced as `IGNORED`
(rather than `.deleted`).

## Acceptance Criteria
- [ ] Introduce a distinct ignore concept (e.g. a new action type such as `IGNORE`/`IGNORED`) emitted by the diff engine when an active path disappears only because an ignore rule now covers it.
- [ ] The ledger should write an `.ignored` marker (or otherwise distinctly record the transition) instead of a `.deleted` marker for such paths.
- [ ] Historical snapshots of that path remain intact and unchanged in `.snapshots/`; only the live tracking state changes.
- [ ] If the ignore rule is later removed, the file should be tracked again (e.g. re-appear as `CREATE`) without corrupting or duplicating history.
- [ ] State projection reducer handling is updated consistently and covered by tests.

## Implementation Plan / Notes
- Discovered while manually testing issue 02 (step 4): adding `.gitignore` patterns caused `DELETE` events for previously-tracked files.
- Only applies when the path still exists on disk but is excluded. A true filesystem deletion should continue to emit `DELETE` as today.
- Requires distinguishing "missing because ignored" from "missing because deleted" during the walk/diff (the walker already knows whether an excluded path exists).
- Should be coordinated with issue 07's integration tests, which will exercise this exact scenario end-to-end.
