# Repository Standards

## Documentation & Projects
- All project documentation resides in `docs/` (e.g., `docs/<module-name>/`).

## File-Based Issue Tracking
We track issues via local files inside module folders.

### Layout
- Issues for a module belong under `docs/<module-name>/issues/`.
- Format: `NN-<kebab-case-description>.md` where `NN` is a zero-padded number (e.g., `01-event-ledger-core-state.md`).

### Issue File Template
Every issue file starts with YAML front-matter with UTC dates:

```markdown
---
id: "NN"
title: "<Title>"
module: "<module-name>"
status: "proposed" # proposed | triage | ready-to-implement | in-progress | completed | abandoned | blocked
branch: ""
created: "YYYY-MM-DDTHH:MM:SSZ"
updated: "YYYY-MM-DDTHH:MM:SSZ"
blocked_by: []
---

# Issue NN: <Title>

## Description
Clear statement of the problem or requirements.

## Acceptance Criteria
- [ ] Requirement 1

## Implementation Plan / Notes
Technical details, architectural considerations, or references to `spec.md`.
```
