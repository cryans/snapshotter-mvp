# Snapshotter Roadmap / Release Scoping

> Build-order and backlog tracking for `snapshotter`. **No code changes live here**; this
> file only records issue status and pick-up order.
>
> Settled design questions are recorded in `docs/snapshotter/spec.md` (see **Design
> Decisions**) rather than here, so the roadmap stays about build order. Latest:
> DD-01 — a file copy is a CREATE, not a COPY (git-mirroring).
>
> Last updated: 2026-09-07T11:49:00Z

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
| 08 | CLI snapshot summary surfaces the dominant action kind | `completed` | Re-scoped to per-line worded action labels; landed `5922163` |
| 09 | Engine diff can miss equal-length modifications within timestamp granularity | `completed` | DD-02 grace-period fast path; tests added |
| 10 | Write README.md with vhs demo GIF | `proposed` | Docs/demo; see `issues/10-…` |

Foundation + lineage + restores + TUI are all landed: **01–07 are done**, **09** is
implemented too (DD-02), and **08** shipped as per-line worded action labels at
`5922163`. The only remaining backlog item is **10** (README + demo GIF). Nothing is
hard-blocked; `10` is a docs/demo task.

## Pick-up order for next session

1. **`10` — README with vhs demo GIF.** Only remaining backlog item. Highest-leverage
   polish; see `issues/10-write-readme-and-vhs-demo.md`.

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
      TUI screenshot / vhs demo; a quickstart that just works; a small architecture diagram.
      Keep the docs/ issue workflow mentioned (it is itself a positive signal).
      Tracked as **issue 10** (`issues/10-write-readme-and-vhs-demo.md`).
- [x] **Issue 09 fix** — landed on `feature/09-engine-diff-modtime-short-circuit` as
      **DD-02** (grace-period fast path + reproduction tests); merge to `main` pending.
- [ ] **LICENSE** — public repo with no license reads as all-rights-reserved / an oversight.
      MIT is the low-friction default. Add only if publishing (this repo is public).
- [ ] **CI build + Makefile `fmt`/`vet`/`tidy`** — green `go vet` + `gofmt` check + `go test`
      signals "maintained". Git hooks were considered but deferred in favour of CI (hooks
      don't ship through version control and duplicate a CI gate).
- [x] **Issue 08 fix** — landed as per-line worded action labels (`5922163`); re-scoped
      from the header-label idea to the delivered output change.
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
