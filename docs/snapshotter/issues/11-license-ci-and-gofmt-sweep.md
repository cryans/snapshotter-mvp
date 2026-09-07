---
id: "11"
title: "Project licensing, CI build gate, and gofmt sweep"
module: "snapshotter"
status: "proposed"
branch: ""
created: "2026-09-07T11:57:51Z"
updated: "2026-09-07T11:57:51Z"
blocked_by: []
---

# Issue 11: Project licensing, CI build gate, and gofmt sweep

> Folded together from the MVP-readiness checklist in `docs/snapshotter/roadmap.md`.
> Three small "makes the repo read as maintained / publishable" housekeeping items.

## Description

A public repo with no license reads as all-rights-reserved or an oversight; a green
`vet` + `fmt` + `test` gate signals "maintained"; and a `gofmt` pass cleans up cosmetic
nits. None change runtime behaviour.

1. **LICENSE** — the repo is public and currently has no license. MIT is the low-friction
   default. (Add only if actually publishing — this repo is public, so yes.)
2. **CI build + Makefile `fmt`/`vet`/`tidy`** — add Makefile targets and a CI gate that run
   `go vet`, a `gofmt` check, and `go test`, so the repo signals "maintained". Git hooks
   were considered but deferred in favour of CI (hooks don't ship through version control
   and duplicate a CI gate).
3. **`gofmt` sweep of `internal/store/*.go`** — fix trailing-whitespace / alignment nits in
   `id.go`, `state.go`, `types.go`, `ledger.go` and their tests, plus `engine_test.go`.

## Acceptance Criteria

- [ ] A `LICENSE` file (MIT) exists at the repo root.
- [ ] Makefile has `fmt` / `vet` / `tidy` targets (consistent with existing `build` / `run` / `test` / `clean`).
- [ ] A CI workflow runs `go vet` + a `gofmt` check + `go test` and must pass on PRs/pushes.
- [ ] `internal/store/*.go` and their tests are `gofmt`-clean (no diffs from `gofmt -l`).
- [ ] No runtime behaviour changes (pure formatting / tooling / docs-of-record).

## Implementation Plan / Notes

- Rooted in the MVP-readiness checklist in `docs/snapshotter/roadmap.md` (LICENSE, CI +
  Makefile `fmt`/`vet`/`tidy`, and `gofmt` sweep items). Git hooks deliberately excluded.
- No interaction with the engine's pure-stdlib invariant or the TUI dependency exception.
