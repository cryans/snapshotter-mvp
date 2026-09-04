# Snapshotter Roadmap / Release Scoping

> Build-order and backlog tracking for `snapshotter`. **No code changes live here**; this
> file only records issue status and pick-up order.
>
> Last updated: 2026-09-04T16:30:00Z

## Backlog status snapshot

| Issue | Title | Status | Notes |
|-------|-------|--------|-------|
| 01 | Event Ledger Core State | `completed` | Foundation |
| 02 | Path Normalization & Ignoring | `completed` | Foundation |
| 03 | Interactive TUI File History Viewer | `completed` | Merged to `main` (`cb9440c`) |
| 04 | Snapshot Rollbacks & Restorations | `completed` | Merged to `main` (`90de1b2`) |
| 05 | Action Lineage & Unique Identifiers | `completed` | Implemented — lineage schema + reducer + tests |
| 06 | Represent newly-ignored files as IGNORED, not DELETE | `completed` | Code already merged to `main`, all ACs `[x]` |
| 07 | End-to-end engine integration tests | `completed` | |
| 08 | CLI snapshot header dominant-action label | `proposed` | Output-only change; independent |
| 09 | Engine diff can miss equal-length modifications within timestamp granularity | `proposed` | Real engine correctness bug; independent |

Foundation + lineage + restores + TUI are all landed: **01–07 are done**. The remaining
backlog is **08** and **09**. Nothing is hard-blocked; both are implementable against the
current schema and are independent of each other.

## Pick-up order for next session

1. **`09` — engine diff modtime short-circuit bug.** This is the highest-value item: a
   genuine correctness bug where the `size == && ModTime.Equal` fast path in
   `internal/store/engine.go` can silently drop an *equal-length* modification written
   within filesystem timestamp granularity. Fixing it (likely by trusting the hash as the
   authoritative signal, or only trusting the mtime fast path when the recorded mtime is
   sufficiently old) is a correctness improvement and should land before further consumers
   of the change stream are built on top of it. Include the requested reproduction test.
2. **`08` — CLI snapshot header dominant-action label.** Small, output-only change in
   `snapshotter.go`. Decides/confirms the parenthetical label precedence for mixed-action
   snapshots (`(modified)` / `(moved)`, and how `CREATE` / `DELETE` / `IGNORED`-only
   snapshots read). Independent of `09` and can be slotted in at any point.

## MVP-readiness checklist

> Recorded plans only — **nothing started** (session note: "don't start new adventures today").
> The repo is public, so the plan favours polish and depth on the interesting core (ULID from
> scratch, move-by-content-hash, append-only ledger, pure-stdlib engine, TUI) over breadth of
> infrastructure, and it must read cleanly to a stranger.

Prioritised by leverage/value for an outside reviewer:

- [ ] **README** (highest leverage). First + often only deep look. Should tell the story:
      core idea and the `Physical Disk Walk → Pure Diff → CommitEvent → Append Ledger →
      Projection` data flow; the hand-built Crockford Base32 ULID; move detection by content
      hash; append-only ledger as source of truth; pure-stdlib engine invariant; a
      TUI screenshot; a quickstart that just works; a small architecture diagram. Keep the
      docs/ issue workflow mentioned (it is itself a positive signal).
- [ ] **Issue 09 fix** — a "trustworthy snapshot tool" with a known silent-data-loss gap
      undercuts the whole thesis to a reviewer. Land before presenting.
- [ ] **LICENSE** — public repo with no license reads as all-rights-reserved / an oversight.
      MIT is the low-friction default. Add only if publishing (this repo is public).
- [ ] **CI build + Makefile `fmt`/`vet`/`tidy`** — green `go vet` + `gofmt` check + `go test`
      signals "maintained". Git hooks were considered but deferred in favour of CI (hooks
      don't ship through version control and duplicate a CI gate).
- [ ] **Issue 08 fix** — cheap polish to close the feature set.
- [ ] **`gofmt` sweep of `internal/store/*.go`** — trailing-whitespace / alignment nits
      (`id.go`, `state.go`, `types.go`, `ledger.go` and their tests, `engine_test.go`).
- [ ] Decide whether **enhanced logging** is worth it for trust (a tool whose job is "you can
      rely on what I recorded"). Polish, not MVP-blocking.

## Recorded as **out of MVP scope** (revisit only with concrete evidence)

Captured here so they aren't re-litigated each session; deliberate deferrals, not forgotten.

- **Docker** — contradicts the local-first, user-space, dependency-minimal philosophy. Snapshots
  target the host filesystem; a container snapshots a throwaway FS — a different use case. Not
  wanted unless a concrete container need appears.
- **Daemon / file-watching** — a direct reversal of the documented design choice ("no
  daemon-based watching; clean unidirectional data flow"). The surest way to blow up an MVP.
- **Multithreaded file walker** — perf optimisation only; pursue if perf testing shows the
  single-threaded walk is the bottleneck on realistic trees.
- **Performance tests** — not until there is evidence the walk is too slow on real targets.
- **git hooks** — redundant if CI exists; opt-in scripts / frameworks add process.
- **Code-review process** — lightweight self-review + PRs at merge boundaries beats a heavy
  review gate for this project.
