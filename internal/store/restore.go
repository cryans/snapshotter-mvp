package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// contentTSLayout is the exact filesystem-safe timestamp layout the ledger uses
// when naming mirrored-tree blobs and markers (see Ledger.Append).
const contentTSLayout = "2006-01-02T15-04-05.000"

// RestoreResult reports what a single-file restore did so the CLI can echo a
// useful summary.
type RestoreResult struct {
	// Path is the slash-normalized relative target that was restored.
	Path string
	// Restored is false when the target already held exactly the committed
	// content (a no-op), true when content was written back to the work tree.
	Restored bool
	// BackupPath is where any divergent pre-restore content was preserved
	// (inside .snapshots/.internal/conflicts/...), or empty if nothing needed
	// backing up (target was absent or already matched).
	BackupPath string
}

// Restore resets a single file to the exact content it held in the snapshot
// identified by commitID.
//
// It is deliberately single-target: only targetPath is materialized and nothing
// else on disk is created, modified, or removed. targetPath must have existed
// (been an active tracked file) at commitID; otherwise it has no restorable
// content and an error is returned.
//
// To honour the mirrored tree as the only content store, Restore replays the
// append-only ledger up to commitID, determines the timestamp of the commit that
// last wrote the target's content at that point in history, and copies the
// matching blob back out of .snapshots/<path>/. If the work tree currently holds
// divergent content at that path it is first preserved under
// .snapshots/.internal/conflicts/ so no local state is silently destroyed.
func (e *Engine) Restore(commitID, targetPath string) (*RestoreResult, error) {
	if strings.TrimSpace(commitID) == "" {
		return nil, fmt.Errorf("restore requires a commit ID")
	}

	target := filepath.ToSlash(filepath.Clean(targetPath))
	if target == "." || target == "/" || target == "" {
		return nil, fmt.Errorf("restore requires a single target path")
	}
	// Guard against escaping the workspace via the mirror.
	if strings.HasPrefix(target, ".snapshots/") || target == ".snapshots" {
		return nil, fmt.Errorf("refusing to restore the tool's own state directory")
	}

	proj, lastWrite, found, err := e.replayUntilCommit(commitID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("commit %q not found in ledger", commitID)
	}

	if _, active := proj.ActiveFiles[target]; !active {
		return nil, fmt.Errorf("%q does not exist in commit %s (nothing to restore)", target, commitID)
	}
	writeTS, ok := lastWrite[target]
	if !ok {
		// Defensive: an active file must always have a recorded write timestamp.
		return nil, fmt.Errorf("no content blob recorded for %q in commit %s", target, commitID)
	}

	blob := findContentBlob(e.snapshotDir, target, writeTS)
	if blob == "" {
		return nil, fmt.Errorf("content blob for %q at commit %s is missing from the mirror", target, commitID)
	}
	content, err := os.ReadFile(blob)
	if err != nil {
		return nil, fmt.Errorf("failed to read content blob: %w", err)
	}

	dest := filepath.Join(e.workDir, filepath.FromSlash(target))

	// No-op when the work tree already holds exactly the committed content.
	if existing, err := os.ReadFile(dest); err == nil && bytes.Equal(existing, content) {
		return &RestoreResult{Path: target}, nil
	}

	// Preserve any divergent content before overwriting it.
	var backupPath string
	if _, err := os.Stat(dest); err == nil {
		backupPath, err = e.backupConflict(dest, target)
		if err != nil {
			return nil, fmt.Errorf("failed to back up divergent content: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create parent dir for %q: %w", target, err)
	}
	if err := os.WriteFile(dest, content, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write restored content to %q: %w", target, err)
	}

	return &RestoreResult{Path: target, Restored: true, BackupPath: backupPath}, nil
}

// replayUntilCommit replays the append-only ledger from the beginning, applying
// commits in order until (and including) the commit whose ID equals commitID.
// It returns:
//   - proj:      the projection exactly as-of that commit (future events ignored),
//   - lastWrite: for every active path, the contentTSLayout timestamp of the
//     commit that last wrote its currently-in-force blob at that point,
//   - found:     whether commitID was seen in the ledger.
func (e *Engine) replayUntilCommit(commitID string) (*Projection, map[string]string, bool, error) {
	proj := NewProjection()
	lastWrite := make(map[string]string)

	f, err := os.Open(e.eventsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return proj, lastWrite, false, nil
		}
		return nil, nil, false, fmt.Errorf("failed to open ledger: %w", err)
	}
	defer f.Close()

	scanner := ledgerScanner(f)
	for scanner.Scan() {
		var commit CommitEvent
		if err := json.Unmarshal(scanner.Bytes(), &commit); err != nil {
			return nil, nil, false, fmt.Errorf("failed to decode ledger event: %w", err)
		}
		ts := commit.Timestamp.Format(contentTSLayout)

		proj.Apply(&commit)
		for _, ch := range commit.Changes {
			switch ch.Action {
			case ActionCreate, ActionModify:
				lastWrite[ch.Path] = ts
			case ActionMove:
				// The source path stops being tracked (it becomes a tombstone);
				// only the destination carries live content from this commit on.
				delete(lastWrite, ch.OldPath)
				lastWrite[ch.Path] = ts
			case ActionDelete, ActionIgnored:
				delete(lastWrite, ch.Path)
			}
		}

		if commit.ID == commitID {
			return proj, lastWrite, true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, false, fmt.Errorf("failed reading ledger: %w", err)
	}

	return proj, lastWrite, false, nil
}

// findContentBlob locates the mirrored content blob that holds a file's version
// as written at writeTS. Content blobs are named <writeTS><ext> inside
// .snapshots/<path>/; marker files (.deleted/.ignored/.moved/.moved_from) carry
// that name as a prefix but never match exactly, so an exact-name match is
// unambiguous. Returns "" when no such blob exists.
func findContentBlob(snapshotDir, relPath, writeTS string) string {
	ext := filepath.Ext(relPath)
	dir := filepath.Join(snapshotDir, filepath.FromSlash(relPath))
	candidate := filepath.Join(dir, writeTS+ext)

	info, err := os.Stat(candidate)
	if err == nil && !info.IsDir() {
		return candidate
	}

	// Defensive fallback: an older commit may have predated millisecond
	// precision, so accept the newest non-marker content snapshot written at or
	// before writeTS.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best := ""
	for _, en := range entries {
		name := en.Name()
		if en.IsDir() || isRestoreMarker(name) {
			continue
		}
		// A content name is <ts><ext>. Isolate the leading ts to compare.
		if !strings.HasSuffix(name, ext) {
			continue
		}
		ts := strings.TrimSuffix(name, ext)
		if ts > writeTS {
			continue
		}
		if best == "" || ts > best {
			best = ts
		}
	}
	if best == "" {
		return ""
	}
	return filepath.Join(dir, best+ext)
}

// backupConflict preserves the divergent current content of dest (which is about
// to be overwritten by a restore) under .snapshots/.internal/conflicts/<run>/<rel>
// so no local state is destroyed without a trace.
func (e *Engine) backupConflict(dest, rel string) (string, error) {
	runDir := filepath.Join(
		e.snapshotDir, ".internal", "conflicts",
		time.Now().UTC().Format(contentTSLayout),
	)
	backup := filepath.Join(runDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		return "", err
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return "", err
	}
	return backup, nil
}

// isRestoreMarker reports whether a mirrored-tree filename is a marker
// (.deleted/.ignored/.moved/.moved_from) rather than real content.
func isRestoreMarker(name string) bool {
	for _, m := range []string{".deleted", ".ignored", ".moved", ".moved_from"} {
		if strings.HasSuffix(name, m) {
			return true
		}
	}
	return false
}
