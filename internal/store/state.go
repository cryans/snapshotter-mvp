package store

import (
	"bufio"
	"encoding/json"
	"os"
)

// Apply updates the projection state with a single commit event.
// It modifies the projection in-place.
func (p *Projection) Apply(commit *CommitEvent) {
	p.LastCommit = commit.ID

	for _, change := range commit.Changes {
		switch change.Action {
		case ActionCreate, ActionModify:
			p.ActiveFiles[change.Path] = FileState{
				Path:         change.Path,
				Hash:         change.Hash,
				Size:         change.Size,
				ModTime:      change.ModTime,
				LastActionID: change.ID,
			}
			// If a file is recreated at a path that was previously tombstoned,
			// we remove the tombstone to reflect its active status.
			delete(p.Tombstones, change.Path)
			// A path being tracked again is no longer in an ignored state.
			delete(p.Ignored, change.Path)

		case ActionIgnored:
			// The file may still exist on disk but is excluded by an ignore
			// rule. Stop tracking it, but do NOT treat it as a deletion: no
			// tombstone, and its historical snapshots stay intact.
			delete(p.ActiveFiles, change.Path)
			p.Ignored[change.Path] = IgnoredEntry{
				Path:      change.Path,
				IgnoredAt: commit.Timestamp,
			}
			delete(p.Tombstones, change.Path)

		case ActionDelete:
			delete(p.ActiveFiles, change.Path)
			p.Tombstones[change.Path] = Tombstone{
				Path:      change.Path,
				DeletedAt: commit.Timestamp,
			}

		case ActionMove:
			oldState, ok := p.ActiveFiles[change.OldPath]
			
			// Remove from old location and mark as deleted (tombstone)
			delete(p.ActiveFiles, change.OldPath)
			p.Tombstones[change.OldPath] = Tombstone{
				Path:      change.OldPath,
				DeletedAt: commit.Timestamp,
			}
			
			// If the event doesn't contain full file details, we carry them over
			// from the existing state prior to the move.
			hash := change.Hash
			if hash == "" && ok {
				hash = oldState.Hash
			}
			size := change.Size
			if size == 0 && ok {
				size = oldState.Size
			}
			modTime := change.ModTime
			if modTime.IsZero() && ok {
				modTime = oldState.ModTime
			}

			// Add to new location
			p.ActiveFiles[change.Path] = FileState{
				Path:         change.Path,
				Hash:         hash,
				Size:         size,
				ModTime:      modTime,
				LastActionID: change.ID,
			}
			delete(p.Tombstones, change.Path)
		}
	}
}

// LoadProjection rebuilds the entire file system state from an event log.
// If the events.jsonl file does not exist, it returns an empty projection.
func LoadProjection(eventsPath string) (*Projection, error) {
	proj := NewProjection()

	f, err := os.Open(eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return proj, nil // Valid state: no events exist yet
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Optionally increase scanner buffer if single events get larger than 64kb
	for scanner.Scan() {
		var commit CommitEvent
		if err := json.Unmarshal(scanner.Bytes(), &commit); err != nil {
			return nil, err
		}
		proj.Apply(&commit)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return proj, nil
}
