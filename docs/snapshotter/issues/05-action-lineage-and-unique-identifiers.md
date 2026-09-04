---
id: "05"
title: "Action Lineage and Unique Identifiers"
module: "snapshotter"
status: "ready-to-implement"
branch: ""
created: "2026-09-04T12:45:00Z"
updated: "2026-09-04T13:29:00Z"
blocked_by: []
---

# Issue 05: Action Lineage and Unique Identifiers

## Description
Introduce a unique identifier for every change action inside a commit event, along with an optional reference to its predecessor's unique identifier (`previous_id`). This links subsequent mutations (e.g., `CREATE` -> `MODIFY` -> `DELETE`) together into an explicit content lineage / graph history for each tracked path.

## Acceptance Criteria
- [ ] Add a unique `id` (Crockford Base32 ULID) to the action/change struct.
- [ ] Add an optional `previous_id` string field to the action/change struct to reference the prior version of that file.
- [ ] Update the state projection reducer to track and resolve the latest action IDs for active files so the engine can look them up and link them as `previous_id` during new commit events.
- [ ] Add rigorous unit tests that verify sequential changes (e.g., CREATE followed by MODIFY) properly propagate and match the correct `previous_id` to their previous state's `id`.

## Ordering & Rationale (roadmap)
- This issue is **foundation work**: it extends the shared change-event schema (`Change`
  gains `id` / `previous_id`) and the projection reducer, so it should land **before** the
  history/restore consumers `03` and `04`. See `docs/snapshotter/roadmap.md`.
- Not blocked by anything; it is the next scheduled item after `01`/`02`.
