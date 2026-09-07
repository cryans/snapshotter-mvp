---
id: "13"
title: "Ledger replay fails with \"bufio.Scanner: token too long\" when a single commit's JSONL line exceeds 64 KiB"
module: "snapshotter"
status: "in-progress"
branch: "fix/13-ledger-bufio-scanner-line-limit"
created: "2026-09-07T13:21:14Z"
updated: "2026-09-07T13:21:14Z"
blocked_by: []
---

# Issue 13: Ledger replay fails with "bufio.Scanner: token too long" when a single commit's JSONL line exceeds 64 KiB

## Description

`events.jsonl` is a JSON Lines ledger where **one serialized `CommitEvent` occupies one
line**, and each commit carries every `Change` for that snapshot (`internal/store/ledger.go`
uses `json.NewEncoder(...).Encode(commit)` per commit). A `Change` stores only metadata —
path, 64-char hex `hash`, size, RFC3339 `mod_time`, and 26-char ULID ids — never file
contents — so a single full-tree commit from a project with even a few hundred files
serializes to well over 64 KiB.

Every ledger replay site reads the file with `bufio.Scanner`, whose default maximum token
(line) size is 64 KiB (`bufio.MaxScanTokenSize`). When any commit line exceeds that, the
scanner aborts with `bufio.Scanner: token too long` and the whole load fails:

- `internal/store/state.go` → `LoadProjection`, which `NewEngine` calls and wraps as
  `failed to load projection` (the message seen at startup). The code even carried the
  comment *"Optionally increase scanner buffer if single events get larger than 64kb"*
  without actually raising the buffer.
- `internal/store/history.go` (per-file history replay for the TUI).
- `internal/store/restore.go` (`replayUntilCommit`).

### Symptom / reproduction

On the first run against an already-populated "real" project, `LoadProjection` finds no
ledger yet and returns an empty projection, so the run **succeeds** and appends a single
CREATE commit containing the whole tree (which can exceed 64 KiB). Nothing reloads the
ledger in the same run, so init "works fine". On the next run the engine calls
`LoadProjection` first, hits that oversized first line, and fails:

```
Failed to initialize snapshot engine: failed to load projection: bufio.Scanner: token too long
```

The data is not corrupt — the reader is simply capped.

## Acceptance Criteria

- [x] `LoadProjection` successfully replays a ledger whose individual commit lines exceed
      64 KiB (previously it failed with `bufio.Scanner: token too long`).
- [x] The same fix applies to the other ledger replay sites (`history.go`,
      `restore.go`) so per-file history and restore do not hit the identical limit.
- [x] A regression test writes a single commit whose serialized line is comfortably over
      64 KiB and asserts the projection still loads and reflects all changes.

## Implementation Plan / Notes

Added a package-level helper in `internal/store/state.go`:

```go
const maxLedgerLine = 256 << 20 // 256 MiB

func ledgerScanner(r io.Reader) *bufio.Scanner {
    s := bufio.NewScanner(r)
    s.Buffer(make([]byte, 64*1024), maxLedgerLine)
    return s
}
```

256 MiB is far beyond any realistic metadata-only commit (a `Change` is ~150–250 bytes of
JSON, so a 256 MiB line would hold well over a million changed files in one snapshot) while
still bounding memory on a single pathological line. All three replay sites now use
`ledgerScanner`. Standard-library only — no new dependencies.

Regression test: `TestLoadProjection_HugeSingleCommit` in `internal/store/state_test.go`
writes a ~1.5 MB single-commit line (5,000 `CREATE` changes with 64-char hashes) and
asserts `LoadProjection` returns all 5,000 active files. The fixture is asserted to exceed
64 KiB so the test genuinely exercises the bug.

### Design note for later

Because a full-tree snapshot is emitted as one line, the ledger can grow large when a tree
changes wholesale. The 256 MiB cap removes the hard failure but, for very large trees, it
may be worth emitting one commit per bounded batch of changes rather than one giant line.
Not required to close this bug.
