package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HistoryEntry is one chronological event in a single path's history. It is
// rendered by the interactive TUI as one selectable row.
//
// A path's history is deliberately per-path and forward-follow only (issue 03):
// entries are reconstructed purely from ledger changes whose target is that
// exact path. Move-away rows carry the new destination so the UI can jump the
// view forward to the file's new location; lineage is intentionally not used to
// trace backwards across renames.
type HistoryEntry struct {
	CommitID  string
	Timestamp time.Time
	Action    ActionType

	// Destination is set on a MOVE row representing a move *away* from the
	// targeted path; it holds the file's new location. Such rows are the ones
	// the TUI follows when Enter is pressed.
	Destination string

	// FromPath is set on a MOVE row representing a move *into* the targeted
	// path; it holds the location the content arrived from.
	FromPath string

	// Current is true only for the newest entry, and only while the file is
	// currently an active (tracked) file at this path. If the path is currently
	// deleted or moved-away no entry is marked current.
	Current bool
}

// FileHistory is the result of resolving a user-supplied target against the
// current workspace and ledger.
type FileHistory struct {
	// Path is the slash-normalized relative path within the workspace.
	Path string

	// OnDisk reports whether the path currently exists as a regular file in the
	// active workspace.
	OnDisk bool

	// Entries is the chronological history for the path, newest first. It is
	// empty when the path has never appeared in the ledger (e.g. it exists on
	// disk but has never been snapshotted, or it has no history at all).
	Entries []HistoryEntry
}

// ResolveTarget normalizes a user-supplied path argument into a slash-normalized
// path that is safely relative to the engine's workspace.
//
// The argument may be absolute or relative to the workspace root (the CLI runs
// with cwd == workspace). Anything that escapes the workspace is rejected.
func (e *Engine) ResolveTarget(arg string) (string, error) {
	if strings.TrimSpace(arg) == "" {
		return "", fmt.Errorf("a file path argument is required")
	}

	clean := filepath.Clean(arg)
	var rel string
	if filepath.IsAbs(clean) {
		r, err := filepath.Rel(e.workDir, clean)
		if err != nil {
			return "", fmt.Errorf("cannot resolve %q against the workspace: %w", arg, err)
		}
		rel = r
	} else {
		rel = clean
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "/" {
		return "", fmt.Errorf("%q is not a file path", arg)
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("%q is outside the workspace", arg)
	}
	// Never inspect the tool's own state or VCS plumbing.
	if rel == ".snapshots" || strings.HasPrefix(rel, ".snapshots/") {
		return "", fmt.Errorf("refusing to inspect the tool's own state directory")
	}
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return "", fmt.Errorf("refusing to inspect VCS metadata")
	}
	return rel, nil
}

// FileHistory resolves a user-supplied target argument and reconstructs that
// path's chronological history by replaying the append-only ledger.
//
// It returns an error only for structurally invalid targets (empty, outside the
// workspace, the tool's own state, etc.). A path that has simply never been
// snapshotted is not an error: FileHistory reports it via OnDisk/empty Entries so
// the caller (the CLI) can decide whether to show an empty history or a
// "not found" message. Per the issue's acceptance criteria the CLI errors only
// when the path is absent from both the active workspace and the ledger.
func (e *Engine) FileHistory(arg string) (*FileHistory, error) {
	target, err := e.ResolveTarget(arg)
	if err != nil {
		return nil, err
	}

	fh := &FileHistory{Path: target}

	// On disk? We only treat a regular file as "present" in the workspace.
	if info, err := os.Stat(filepath.Join(e.workDir, filepath.FromSlash(target))); err == nil && info.Mode().IsRegular() {
		fh.OnDisk = true
	}

	// Replay the ledger collecting every change that touched the exact path.
	var entries []HistoryEntry
	f, err := os.Open(e.eventsPath())
	if err != nil {
		if os.IsNotExist(err) {
			fh.Entries = entries
			return fh, nil
		}
		return nil, fmt.Errorf("failed to open ledger: %w", err)
	}
	defer f.Close()

	scanner := ledgerScanner(f)
	for scanner.Scan() {
		var commit CommitEvent
		if err := json.Unmarshal(scanner.Bytes(), &commit); err != nil {
			return nil, fmt.Errorf("failed to decode ledger event: %w", err)
		}
		for _, ch := range commit.Changes {
			switch ch.Action {
			case ActionCreate, ActionModify:
				if ch.Path != target {
					continue
				}
				entries = append(entries, HistoryEntry{
					CommitID:  commit.ID,
					Timestamp: commit.Timestamp,
					Action:    ch.Action,
				})
			case ActionMove:
				switch {
				case ch.OldPath == target: // moved away -> followable row
					entries = append(entries, HistoryEntry{
						CommitID:    commit.ID,
						Timestamp:   commit.Timestamp,
						Action:      ch.Action,
						Destination: ch.Path,
					})
				case ch.Path == target: // moved into this path -> content row
					entries = append(entries, HistoryEntry{
						CommitID:  commit.ID,
						Timestamp: commit.Timestamp,
						Action:    ch.Action,
						FromPath:  ch.OldPath,
					})
				}
			case ActionDelete:
				if ch.Path != target {
					continue
				}
				entries = append(entries, HistoryEntry{
					CommitID:  commit.ID,
					Timestamp: commit.Timestamp,
					Action:    ch.Action,
				})
			case ActionIgnored:
				if ch.Path != target {
					continue
				}
				entries = append(entries, HistoryEntry{
					CommitID:  commit.ID,
					Timestamp: commit.Timestamp,
					Action:    ch.Action,
				})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed reading ledger: %w", err)
	}

	// Mark the newest entry as current when the file is still tracked now.
	// entries is currently oldest->newest (ledger append order); the newest
	// entry is the final element. Only an action that leaves the file active at
	// this path (a create/modify or a move-into) may carry the current marker,
	// so we never label a stale move-away/delete/ignore row as current.
	if n := len(entries); n > 0 {
		newest := entries[n-1]
		leavesActive := newest.Action == ActionCreate || newest.Action == ActionModify ||
			(newest.Action == ActionMove && newest.FromPath != "")
		if _, active := e.state.ActiveFiles[target]; leavesActive && active {
			entries[n-1].Current = true
		}
	}

	// Reverse to newest-first, keeping ledger tie-break for equal timestamps.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	fh.Entries = entries
	return fh, nil
}
