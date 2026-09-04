package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Engine orchestrates the snapshot process by integrating the ledger, state, and diff logic.
type Engine struct {
	workDir string
	ledger  *Ledger
	state   *Projection
}

// NewEngine initializes a new Engine.
func NewEngine(workDir, snapshotDir string) (*Engine, error) {
	ledger := NewLedger(workDir, snapshotDir)
	
	eventsPath := filepath.Join(snapshotDir, ".internal", "events.jsonl")
	state, err := LoadProjection(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load projection: %w", err)
	}

	return &Engine{
		workDir: workDir,
		ledger:  ledger,
		state:   state,
	}, nil
}

// State returns the current in-memory view of the file system.
func (e *Engine) State() *Projection {
	return e.state
}

// Snapshot scans the directory, diffs against the projection, appends a commit, and updates state.
func (e *Engine) Snapshot(dir string) (*CommitEvent, error) {
	changes, err := e.diff(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to compute diff: %w", err)
	}

	// If there are no changes, we don't need to commit anything
	if len(changes) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	commit := &CommitEvent{
		ID:        NewCommitID(now),
		Timestamp: now,
		Message:   "Snapshot",
		Changes:   changes,
	}

	if err := e.ledger.Append(commit); err != nil {
		return nil, fmt.Errorf("failed to append ledger: %w", err)
	}

	e.state.Apply(commit)
	return commit, nil
}

func (e *Engine) diff(dir string) ([]Change, error) {
	type diskFile struct {
		path string
		info os.FileInfo
		hash string
	}
	diskFiles := make(map[string]*diskFile)

	// 1. Walk the physical disk, honoring .gitignore rules and hard exclusions.
	tracked, err := newWalker(dir).collect()
	if err != nil {
		return nil, err
	}
	for _, tf := range tracked {
		info, err := os.Stat(tf.abs)
		if err != nil {
			return nil, err
		}
		diskFiles[tf.rel] = &diskFile{path: tf.abs, info: info}
	}

	var changes []Change
	
	// 2. Identify missing active files (potential Deletes or Moves)
	missingActive := make(map[string]FileState)
	for path, state := range e.state.ActiveFiles {
		if _, exists := diskFiles[path]; !exists {
			missingActive[path] = state
		}
	}
	
	// Index missing files by hash to quickly find move targets
	missingByHash := make(map[string]FileState)
	for _, state := range missingActive {
		missingByHash[state.Hash] = state
	}

	// 3. Evaluate existing files (Creates, Modifies, Moves, or Unchanged)
	for relPath, df := range diskFiles {
		state, exists := e.state.ActiveFiles[relPath]
		
		// Optimization: if size and modtime exactly match, skip hashing
		if exists && state.Size == df.info.Size() && state.ModTime.Equal(df.info.ModTime()) {
			continue
		}
		
		// Hash the file
		hash, err := hashFileInternal(df.path)
		if err != nil {
			return nil, err
		}
		df.hash = hash
		
		if exists {
			// Modified
			if state.Hash != hash {
				changes = append(changes, Change{
					Action:  ActionModify,
					Path:    relPath,
					Hash:    hash,
					Size:    df.info.Size(),
					ModTime: df.info.ModTime(),
				})
			}
		} else {
			// Check if it's a move by correlating the hash
			if oldState, ok := missingByHash[hash]; ok {
				changes = append(changes, Change{
					Action:  ActionMove,
					Path:    relPath,
					OldPath: oldState.Path,
					Hash:    hash,
					Size:    df.info.Size(),
					ModTime: df.info.ModTime(),
				})
				delete(missingActive, oldState.Path) // consumed by move
				delete(missingByHash, hash)
			} else {
				// Truly a new file
				changes = append(changes, Change{
					Action:  ActionCreate,
					Path:    relPath,
					Hash:    hash,
					Size:    df.info.Size(),
					ModTime: df.info.ModTime(),
				})
			}
		}
	}

	// 4. Remaining missing files are true Deletes
	for path := range missingActive {
		changes = append(changes, Change{
			Action: ActionDelete,
			Path:   path,
		})
	}

	return changes, nil
}

func hashFileInternal(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
