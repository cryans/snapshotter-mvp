package store

import (
	"time"
)

// ActionType represents the type of change applied to a file.
type ActionType string

const (
	ActionCreate  ActionType = "CREATE"
	ActionModify  ActionType = "MODIFY"
	ActionMove    ActionType = "MOVE"
	ActionDelete  ActionType = "DELETE"
	ActionIgnored ActionType = "IGNORED"
)

// Change represents a single modification to the file system state.
//
// ID is a unique, statelessly-generated 26-character Crockford Base32 ULID that
// identifies this action. PreviousID optionally references the ID of the action
// that produced the immediately-previous version of this logical file, forming
// an explicit content lineage (e.g. CREATE -> MODIFY -> DELETE). A CREATE that
// starts a fresh lineage, or a change whose predecessor predates lineage
// tracking (legacy ledger entries), leaves PreviousID empty.
type Change struct {
	Action     ActionType `json:"action"`
	Path       string     `json:"path"`
	Hash       string     `json:"hash,omitempty"`
	OldPath    string     `json:"old_path,omitempty"` // Only used for MOVE actions
	Size       int64      `json:"size,omitempty"`
	ModTime    time.Time  `json:"mod_time,omitempty"`
	ID         string     `json:"id"`
	PreviousID string     `json:"previous_id,omitempty"`
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
	// LastActionID is the ID of the action that produced the current version of
	// this active file. The engine reads it to link the next mutation (MODIFY,
	// MOVE, DELETE, IGNORED) via Change.PreviousID. Empty for legacy entries
	// that predate lineage tracking.
	LastActionID string `json:"last_action_id,omitempty"`
}

// Tombstone marks a previously tracked file that has been deleted.
type Tombstone struct {
	Path      string    `json:"path"`
	DeletedAt time.Time `json:"deleted_at"`
}

// IgnoredEntry records that a file is currently excluded by an ignore rule.
// Unlike a Tombstone (a real deletion) the content may still exist on disk.
type IgnoredEntry struct {
	Path     string    `json:"path"`
	IgnoredAt time.Time `json:"ignored_at"`
}

// Projection is the materialized in-memory view of the file system state
// rebuilt from the append-only ledger.
type Projection struct {
	ActiveFiles map[string]FileState
	Tombstones  map[string]Tombstone
	Ignored     map[string]IgnoredEntry
	LastCommit  string
}

// NewProjection creates an empty state projection.
func NewProjection() *Projection {
	return &Projection{
		ActiveFiles: make(map[string]FileState),
		Tombstones:  make(map[string]Tombstone),
		Ignored:     make(map[string]IgnoredEntry),
	}
}
