package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProjection_Apply(t *testing.T) {
	proj := NewProjection()

	now := time.Now()
	commit1 := &CommitEvent{
		ID:        "01H9QY...",
		Timestamp: now,
		Changes: []Change{
			{
				Action:  ActionCreate,
				Path:    "docs.md",
				Hash:    "abc",
				Size:    100,
				ModTime: now,
			},
		},
	}

	proj.Apply(commit1)

	if proj.LastCommit != "01H9QY..." {
		t.Errorf("Expected LastCommit to be updated")
	}
	if len(proj.ActiveFiles) != 1 {
		t.Fatalf("Expected 1 active file")
	}
	file, ok := proj.ActiveFiles["docs.md"]
	if !ok || file.Hash != "abc" {
		t.Errorf("Expected docs.md to be created correctly")
	}

	// Move the file
	commit2 := &CommitEvent{
		ID:        "01H9QZ...",
		Timestamp: now.Add(time.Minute),
		Changes: []Change{
			{
				Action:  ActionMove,
				Path:    "readme.md",
				OldPath: "docs.md",
			},
		},
	}
	proj.Apply(commit2)

	if _, ok := proj.ActiveFiles["docs.md"]; ok {
		t.Errorf("Expected docs.md to be removed from active files")
	}
	if _, ok := proj.ActiveFiles["readme.md"]; !ok {
		t.Errorf("Expected readme.md to be present in active files")
	}
	
	movedFile := proj.ActiveFiles["readme.md"]
	if movedFile.Hash != "abc" || movedFile.Size != 100 {
		t.Errorf("Expected move to carry over state properties, got hash=%s, size=%d", movedFile.Hash, movedFile.Size)
	}

	if _, ok := proj.Tombstones["docs.md"]; !ok {
		t.Errorf("Expected docs.md to be tombstoned after move")
	}

	// Delete the file
	commit3 := &CommitEvent{
		ID:        "01H9QA...",
		Timestamp: now.Add(2 * time.Minute),
		Changes: []Change{
			{
				Action: ActionDelete,
				Path:   "readme.md",
			},
		},
	}
	proj.Apply(commit3)

	if len(proj.ActiveFiles) != 0 {
		t.Errorf("Expected 0 active files after deletion")
	}
	if _, ok := proj.Tombstones["readme.md"]; !ok {
		t.Errorf("Expected readme.md to be tombstoned")
	}
}

func TestLoadProjection(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")

	// 1. Missing file should yield empty projection, not an error
	proj, err := LoadProjection(eventsPath)
	if err != nil {
		t.Fatalf("LoadProjection on missing file returned error: %v", err)
	}
	if len(proj.ActiveFiles) != 0 {
		t.Errorf("Expected empty active files on fresh load")
	}

	// 2. Write some events
	f, err := os.Create(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.Encode(&CommitEvent{
		ID: "COMMIT-1",
		Changes: []Change{
			{Action: ActionCreate, Path: "file1.txt", Hash: "aaa", Size: 10},
		},
	})
	enc.Encode(&CommitEvent{
		ID: "COMMIT-2",
		Changes: []Change{
			{Action: ActionModify, Path: "file1.txt", Hash: "bbb", Size: 20},
			{Action: ActionCreate, Path: "file2.txt", Hash: "ccc", Size: 30},
		},
	})

	// 3. Load from valid file
	proj2, err := LoadProjection(eventsPath)
	if err != nil {
		t.Fatalf("LoadProjection failed: %v", err)
	}

	if proj2.LastCommit != "COMMIT-2" {
		t.Errorf("Expected LastCommit to be COMMIT-2, got %s", proj2.LastCommit)
	}
	if len(proj2.ActiveFiles) != 2 {
		t.Errorf("Expected 2 active files")
	}
	if proj2.ActiveFiles["file1.txt"].Hash != "bbb" {
		t.Errorf("Expected file1.txt hash to be 'bbb'")
	}
}
