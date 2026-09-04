# Snapshotter Roadmap / Release Scoping

> Refinement session on `refinement/issue-05-lineage-prep` — agreed build order for the
> open backlog, grounded in real dependencies (hard blockers vs shared-foundation
> sequencing). **No code changes live here**; this branch only refines issues and
> roadmap `.md` files.
>
> Last updated: 2026-09-04T13:56:23Z

## Backlog status snapshot

| Issue | Title | Status | Notes |
|-------|-------|--------|-------|
| 01 | Event Ledger Core State | `completed` | Foundation |
| 02 | Path Normalization & Ignoring | `completed` | Foundation |
| 03 | Interactive TUI File History Viewer | `ready-to-implement` | After `05`; `blocked_by [01, 02]` satisfied |
| 04 | Snapshot Rollbacks & Restorations | `ready-to-implement` | After `05` |
| 05 | Action Lineage & Unique Identifiers | `completed` | Implemented — lineage schema + reducer + tests |
| 06 | Represent newly-ignored files as IGNORED, not DELETE | `completed` | Code already merged to `main`, all ACs `[x]` |
| 07 | End-to-end engine integration tests | `completed` | |
| 08 | CLI snapshot header dominant-action label | `proposed` | Output-only change |

## Dependency facts (hard blockers)

- Hard dependencies only:
  - `03` ← `{01, 02}` (both satisfied).
  - `04` ← `{01}` (satisfied).
  - `05` ← `{}`.
  - `08` ← `{}`.
- **`03` and `04` are *not* hard-blocked by `05`.** Each is implementable and testable
  against the current change-event model (commit ULID + timestamp + path + move markers)
  without `id`/`previous_id`.

## Shared-foundation sequencing (why `05` goes first)

Even though it is not a hard blocker, `05` should be scheduled **before** `03` and `04`:

- `05` mutates the **shared change-event schema** (`Change` gains `id` / `previous_id`)
  and the **projection reducer**, plus ledger/tests on that model.
- `03`'s history traversal and `04`'s restore resolution are consumers of that same event
  model. Writing them after `05` means they target the final schema and can use lineage
  (trace a logical file across renames; restore content that lived at moved paths) instead
  of retrofitting it later.
- Therefore: **`05` → then `{03, 04}`**. `08` is independent and may land any time.

## Proposed build order

1. `01`, `02`, `05`, `06`, `07` — done (foundation + lineage + tests).
2. **`05` — Action Lineage** — change-event schema (`Change` gains `id` / `previous_id`)
   and projection-reducer foundation.
3. `04` and/or `03` — restore & TUI history, now lineage-capable. Relative order between
   them is a product-priority call (data recovery vs. browsing) — decide at pick-up.
   **These are now the next items to pick up after `05`.**
4. `08` — small independent CLI-output improvement (can be slotted anywhere, incl. now).
5. Housekeeping: `06` flipped to `completed`; `05` flipped to `completed` this session.
