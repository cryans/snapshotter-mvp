# Snapshotter Roadmap / Release Scoping

> Build-order and backlog tracking for `snapshotter`. **No code changes live here**; this
> file only records issue status and pick-up order.
>
> Settled design questions are recorded in `docs/snapshotter/spec.md` (see **Design
> Decisions**) rather than here, so the roadmap stays about build order. Latest:
> DD-01 — a file copy is a CREATE, not a COPY (git-mirroring).
>
> Last updated: 2026-09-07T12:01:00Z

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
| 11 | Project licensing, CI build gate, and gofmt sweep | `proposed` | LICENSE + Makefile `fmt`/`vet`/`tidy` + `gofmt` sweep |
| 12 | Decide whether enhanced logging is worth adding | `proposed` | Decision; polish, not MVP-blocking |

Foundation + lineage + restores + TUI are all landed: **01–07 are done**, **09** is
implemented too (DD-02), and **08** shipped as per-line worded action labels at
`5922163`. The remaining backlog — triaged on `backlog-planning` — is **11** (LICENSE +
CI + gofmt sweep), **12** (enhanced-logging decision), and the README write (**10**).
Nothing is hard-blocked; pick-up priority is 11 → 12 → 10.

## Pick-up order for next session

Triaged on `backlog-planning`. Remaining open work, in suggested build order:

1. **`11` — Project licensing, CI build gate, and gofmt sweep.** Public-repo
   publishability / "maintained" signal: MIT `LICENSE`, Makefile `fmt`/`vet`/`tidy` + a
   CI `vet`/`gofmt`/`test` gate, and a `gofmt` sweep of `internal/store/*.go`.
   See `issues/11-license-ci-and-gofmt-sweep.md`.
2. **`12` — Decide whether enhanced logging is worth adding.** Decision item; polish,
   not MVP-blocking. See `issues/12-enhanced-logging-decision.md`.
3. **`10` — README with vhs demo GIF.** Headline documentation task, done last. See
   `issues/10-write-readme-and-vhs-demo.md`.

Nothing is hard-blocked. Pick-up priority is **11 → 12 → 10**.

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
- [x] **Issue 08 fix** — landed as per-line worded action labels (`5922163`); re-scoped
      from the header-label idea to the delivered output change.
- **LICENSE / CI + Makefile `fmt`/`vet`/`tidy` / `gofmt` sweep** — folded into
      **issue 11** (`issues/11-license-ci-and-gofmt-sweep.md`).
- **Enhanced logging decision** — tracked as **issue 12**
      (`issues/12-enhanced-logging-decision.md`); polish, not MVP-blocking.

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
