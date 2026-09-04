package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Ledger is responsible for persisting commit events and their associated file states.
type Ledger struct {
	workDir     string
	snapshotDir string
}

// NewLedger creates a new Ledger instance.
func NewLedger(workDir, snapshotDir string) *Ledger {
	return &Ledger{
		workDir:     workDir,
		snapshotDir: snapshotDir,
	}
}

// Append writes the commit event to the JSONL ledger and writes the file states
// into the human-readable mirrored tree.
func (l *Ledger) Append(commit *CommitEvent) error {
	// 1. Append to JSONL
	internalDir := filepath.Join(l.snapshotDir, ".internal")
	if err := os.MkdirAll(internalDir, 0755); err != nil {
		return fmt.Errorf("failed to create internal dir: %w", err)
	}

	eventsPath := filepath.Join(internalDir, "events.jsonl")
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open events.jsonl: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(commit); err != nil {
		return fmt.Errorf("failed to encode commit: %w", err)
	}

	// 2. Write mirrored tree
	ts := commit.Timestamp.Format("2006-01-02T15-04-05.000") // safe characters for filesystem
	
	for _, change := range commit.Changes {
		if err := l.writeMirroredTree(change, ts); err != nil {
			return fmt.Errorf("failed to write mirrored tree for %s: %w", change.Path, err)
		}
	}

	return nil
}

func (l *Ledger) writeMirroredTree(change Change, timestamp string) error {
	switch change.Action {
	case ActionCreate, ActionModify:
		return l.copyToSnapshot(change.Path, change.Path, timestamp, "")
		
	case ActionDelete:
		targetDir := filepath.Join(l.snapshotDir, change.Path)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
		ext := filepath.Ext(change.Path)
		targetFile := filepath.Join(targetDir, timestamp+ext+".deleted")
		return os.WriteFile(targetFile, nil, 0644)

	case ActionMove:
		// 1. Write .moved file in old path
		oldTargetDir := filepath.Join(l.snapshotDir, change.OldPath)
		if err := os.MkdirAll(oldTargetDir, 0755); err != nil {
			return err
		}
		oldExt := filepath.Ext(change.OldPath)
		movedMarker := filepath.Join(oldTargetDir, timestamp+oldExt+".moved")
		if err := os.WriteFile(movedMarker, []byte(change.Path), 0644); err != nil {
			return err
		}

		// 2. Write .moved_from file in new path
		newTargetDir := filepath.Join(l.snapshotDir, change.Path)
		if err := os.MkdirAll(newTargetDir, 0755); err != nil {
			return err
		}
		newExt := filepath.Ext(change.Path)
		movedFromMarker := filepath.Join(newTargetDir, timestamp+newExt+".moved_from")
		if err := os.WriteFile(movedFromMarker, []byte(change.OldPath), 0644); err != nil {
			return err
		}

		// 3. Write actual content
		return l.copyToSnapshot(change.Path, change.Path, timestamp, "")
	
	default:
		return fmt.Errorf("unknown action: %s", change.Action)
	}
}

// copyToSnapshot copies a file from the work tree to the snapshot mirrored tree.
func (l *Ledger) copyToSnapshot(srcPath, dstPath, timestamp, suffix string) error {
	srcFull := filepath.Join(l.workDir, srcPath)
	dstDir := filepath.Join(l.snapshotDir, dstPath)
	
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	ext := filepath.Ext(dstPath)
	dstFull := filepath.Join(dstDir, timestamp+ext+suffix)

	src, err := os.Open(srcFull)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstFull)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}
