package store

import (
	"time"
)

// ActionType represents the type of change applied to a file.
type ActionType string

const (
	ActionCreate ActionType = "CREATE"
	ActionModify ActionType = "MODIFY"
	ActionMove   ActionType = "MOVE"
	ActionDelete ActionType = "DELETE"
)

// Change represents a single modification to the file system state.
type Change struct {
	Action  ActionType `json:"action"`
	Path    string     `json:"path"`
	Hash    string     `json:"hash,omitempty"`
	OldPath string     `json:"old_path,omitempty"` // Only used for MOVE actions
	Size    int64      `json:"size,omitempty"`
	ModTime time.Time  `json:"mod_time,omitempty"`
}

// CommitEvent represents a complete snapshot transaction.
type CommitEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Changes   []Change  `json:"changes"`
}

// FileState holds the metadata for a file currently tracked in the snapshot.
type FileState struct {
	Path    string    `json:"path"`
	Hash    string    `json:"hash"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// Tombstone marks a previously tracked file that has been deleted.
type Tombstone struct {
	Path      string    `json:"path"`
	DeletedAt time.Time `json:"deleted_at"`
}

// Projection is the materialized in-memory view of the file system state
// rebuilt from the append-only ledger.
type Projection struct {
	ActiveFiles map[string]FileState
	Tombstones  map[string]Tombstone
	LastCommit  string
}

// NewProjection creates an empty state projection.
func NewProjection() *Projection {
	return &Projection{
		ActiveFiles: make(map[string]FileState),
		Tombstones:  make(map[string]Tombstone),
	}
}
