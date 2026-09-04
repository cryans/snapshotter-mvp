---
id: "10"
title: "Write README.md with vhs demo GIF"
module: "snapshotter"
status: "proposed"
branch: ""
created: "2026-09-04T15:43:25Z"
updated: "2026-09-04T15:43:25Z"
blocked_by: []
---

# Issue 10: Write README.md with vhs demo GIF

## Description

There is currently **no `README.md`**. Add a root README that introduces the project,
shows it in action, and explains how it was built. Pair it with a **vhs** terminal-demo GIF
plus a Makefile target to (re)generate the GIF from a committed vhs tape.

## Proposed README structure (top to bottom)

1. **One-liner** — a single sentence stating what `snapshotter` is
   (local-first, deterministic file snapshotting with a terminal history viewer).
2. **Demo GIF** — a vhs-recorded terminal walkthrough, shown immediately after the one-liner.
3. **How it was built** — first built as a quick prototype, then driven by
   **DeepSeek V4 + DeepSeek Harness**.
4. **Rationale** — we wanted a small project to test whether *agentic engineering* is
   possible; a working prototype was essential as grounding for the agent-driven follow-on.
5. Getting Started / Usage and architecture are open decisions (see Notes).

## vhs demo + Makefile

- Add a vhs tape file (e.g. `demo.tape`) that records the walkthrough.
- Add a Makefile command to build the GIF from the tape (e.g. `make demo` → `vhs demo.tape`
  producing the README GIF).
- The vhs recording must **use a dark theme** (`Set Theme "Dark"` / a dark custom theme).

## Acceptance Criteria
- [ ] A `README.md` exists at the repo root.
- [ ] README opens with a one-line description of what the project is.
- [ ] A demo GIF appears immediately after the one-liner.
- [ ] The README explains the build history (prototype first, then DeepSeek V4 + DeepSeek Harness).
- [ ] The README states the rationale (small project to test whether agentic engineering was
      possible; the prototype was essential).
- [ ] A vhs tape is committed and a Makefile target builds the GIF from it in a dark theme.

## Implementation Plan / Notes

- **vhs**: install/verify `vhs`; write `demo.tape` with `Set Theme "Dark"`, a `Type`/`Sleep`
  script showing `snapshotter <file>` (TUI history view) and optionally a `restore`. GIF output
  referenced by the README.
- **Makefile target**: keep consistent with existing targets (`build`, `run`, `test`, `clean`).
- **Getting started?** Yes — recommended. A short "Getting Started / Usage" after the intro
  (build via `make build`, snapshot a file, open the history TUI, restore a version) is the
  natural entry point since there is no other usage doc aimed at end users.
- **Architecture link?** The closest existing doc is `docs/snapshotter/spec.md` (Overview +
  Architectural Invariants). Recommend a short architecture paragraph/diagram in the README that
  links `spec.md` for the invariants. `AGENTS.md` holds the fuller architecture but is written for
  AI agents and is **not** appropriate to link from a README. A dedicated `ARCHITECTURE.md` is an
  optional future follow-up.
- Note the README item in `docs/snapshotter/roadmap.md` (MVP-readiness checklist) — link/close it
  when this lands.
