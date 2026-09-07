---
id: "12"
title: "Decide whether enhanced logging is worth adding"
module: "snapshotter"
status: "proposed"
branch: ""
created: "2026-09-07T11:57:51Z"
updated: "2026-09-07T11:57:51Z"
blocked_by: []
---

# Issue 12: Decide whether enhanced logging is worth adding

> Polish item, **not MVP-blocking**. Split out as its own decision issue so it isn't lost
> under the code-facing cleanup issues.

## Description

The tool's job is "you can rely on what I recorded", so there is an argument for richer
logging as a trust signal. Decide whether enhanced logging is worth it, and if so what
shape it takes (verbosity levels, what gets logged when, log destination). This is a
decision + optional small implementation — polish, not MVP-blocking.

## Acceptance Criteria

- [ ] A decision is recorded (e.g. in `docs/snapshotter/spec.md` or as a note here) on whether to add enhanced logging.
- [ ] If adopted, the scope/levels/destination are defined.

## Implementation Plan / Notes

- Keep it separate from issue 11 (LICENSE/CI/gofmt) since it is a decision with optional
  behavioural change rather than pure housekeeping.
- No engine invariant or layout changes unless explicitly scoped later.
