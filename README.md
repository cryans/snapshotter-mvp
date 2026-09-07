# snapshotter

A local-first, deterministic file snapshotting engine with an interactive terminal
history viewer. Walk a directory, snapshot it, keep an append-only ledger of every
change, and browse or roll back any file's history — all in user-space.

<p align="center">
  <img src="demo.gif" alt="snapshotter demo" width="100%">
</p>

## Demo

The recording above walks through the whole flow against a scratch project in
`/tmp/demo`: create files and sub-directories, take a first snapshot (everything
reported as **created**), edit one file and snapshot again (**modified**), inspect the
mirrored `.snapshots/` layout, delete a file (**deleted**), query the append-only event
log with `jq`, move a file (**moved**), then open the interactive viewer for a file and
roll it back to an earlier version.

To regenerate the GIF yourself you need [`vhs`](https://github.com/charmbracelet/vhs)
(and its `ffmpeg` + `ttyd` dependencies). The recording is a committed tape:

```sh
make demo   # builds bin/snapshotter and runs: PATH="$PWD/bin:$PATH" vhs demo.tape
```

## Getting started

```sh
make build                # produces ./bin/snapshotter
cd /path/to/a/project
snapshotter               # snapshot the current directory
snapshotter <file>        # open the interactive history viewer for a file
snapshotter restore <commit-id> <path>   # restore one file to a commit
```

Every snapshot lives under a `.snapshots/` directory at the workspace root: a
mirrored tree of timestamped file copies plus an append-only event ledger at
`.snapshots/.internal/events.jsonl`.

## How it was built

`snapshotter` was first built quickly as a small working prototype, then developed
further almost entirely by an AI coding agent (**DeepSeek V4** running in the
**DeepSeek Harness**). Feature work is driven by file-based issues in
[`docs/snapshotter/`](docs/snapshotter/), and each issue records the decision trail as
it was resolved.

## Rationale

This is a small project chosen to test whether *agentic engineering* is possible — can
a capable model carry a nontrivial codebase from a prototype through iterative,
issue-driven development? A working prototype was essential: it gave the agent concrete
grounding and a real binary to exercise as the scope grew (rich CLI, restore/rollback,
move detection, and an interactive history TUI).

## Architecture

The engine lives in `internal/store` and is standard-library Go only (one documented
exception: `.gitignore` parsing). It runs on a clean, unidirectional data flow:

`Physical Disk Walk → Pure Diff Engine → CommitEvent → Append Ledger & Blobs → Update Projection`

Deeper background and the architectural invariants are in
[`docs/snapshotter/spec.md`](docs/snapshotter/spec.md).
