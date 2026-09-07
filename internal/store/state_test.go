package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestProjection_IgnoredLifecycle(t *testing.T) {
	proj := NewProjection()
	now := time.Now()

	// Track a file.
	proj.Apply(&CommitEvent{
		ID: "C1", Timestamp: now,
		Changes: []Change{{Action: ActionCreate, Path: "secret.log", Hash: "abc", Size: 10}},
	})
	if _, ok := proj.ActiveFiles["secret.log"]; !ok {
		t.Fatalf("file should be active after CREATE")
	}

	// It becomes ignored -> removed from active, added to ignored, no tombstone.
	proj.Apply(&CommitEvent{
		ID: "C2", Timestamp: now.Add(time.Minute),
		Changes: []Change{{Action: ActionIgnored, Path: "secret.log"}},
	})
	if _, ok := proj.ActiveFiles["secret.log"]; ok {
		t.Errorf("ignored file should not be active")
	}
	if _, ok := proj.Tombstones["secret.log"]; ok {
		t.Errorf("ignored file should not create a tombstone")
	}
	if _, ok := proj.Ignored["secret.log"]; !ok {
		t.Errorf("ignored file should be recorded in the ignored set")
	}

	// Later the rule is removed and the file is tracked again -> CREATE clears ignored.
	proj.Apply(&CommitEvent{
		ID: "C3", Timestamp: now.Add(2 * time.Minute),
		Changes: []Change{{Action: ActionCreate, Path: "secret.log", Hash: "abc", Size: 10}},
	})
	if _, ok := proj.ActiveFiles["secret.log"]; !ok {
		t.Errorf("file should be active again after re-tracking")
	}
	if _, ok := proj.Ignored["secret.log"]; ok {
		t.Errorf("re-tracked file should be removed from the ignored set")
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

func TestLoadProjection_HugeSingleCommit(t *testing.T) {
	// A single snapshot commit stores ALL of its changes on one JSONL line. A
	// full-tree first-run snapshot of a real project can therefore serialize to
	// well over bufio.Scanner's 64 KiB default, which used to make every
	// subsequent replay fail with "bufio.Scanner: token too long".
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")

	f, err := os.Create(eventsPath)
	if err != nil {
		t.Fatal(err)
	}

	const fileCount = 5000
	dirPrefix := strings.Repeat("deep/", 30)
	changes := make([]Change, 0, fileCount)
	for i := 0; i < fileCount; i++ {
		p := fmt.Sprintf("pkg/%sfile-%05d.txt", dirPrefix, i)
		changes = append(changes, Change{
			Action: ActionCreate, Path: p,
			Hash: strings.Repeat("ab", 32), // 64-char sha256 hex
			Size: 100, ModTime: time.Now(),
			ID: "01H9QY8V8GX6VXJY0N3R2M7K4Q",
		})
	}
	commit := &CommitEvent{ID: "HUGE-1", Timestamp: time.Now(), Changes: changes}
	if err := json.NewEncoder(f).Encode(commit); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	if info, _ := os.Stat(eventsPath); info.Size() <= 64*1024 {
		t.Fatalf("test fixture line is only %d bytes; must exceed 64 KiB to exercise the bug", info.Size())
	}

	proj, err := LoadProjection(eventsPath)
	if err != nil {
		t.Fatalf("LoadProjection rejected an oversized single commit line: %v", err)
	}
	if len(proj.ActiveFiles) != fileCount {
		t.Errorf("expected %d active files, got %d", fileCount, len(proj.ActiveFiles))
	}
}

func TestProjection_TracksLastActionID(t *testing.T) {
	proj := NewProjection()
	now := time.Now()

	// A CREATE lands the file active and records its action ID.
	proj.Apply(&CommitEvent{
		ID: "C1", Timestamp: now,
		Changes: []Change{{
			Action: ActionCreate, Path: "a.txt", Hash: "h1", ID: "ACTION-CREATE",
		}},
	})
	if got := proj.ActiveFiles["a.txt"].LastActionID; got != "ACTION-CREATE" {
		t.Errorf("after CREATE, LastActionID = %q, want ACTION-CREATE", got)
	}

	// A subsequent MODIFY advances the recorded last action ID.
	proj.Apply(&CommitEvent{
		ID: "C2", Timestamp: now.Add(time.Minute),
		Changes: []Change{{
			Action: ActionModify, Path: "a.txt", Hash: "h2", ID: "ACTION-MODIFY",
		}},
	})
	if got := proj.ActiveFiles["a.txt"].LastActionID; got != "ACTION-MODIFY" {
		t.Errorf("after MODIFY, LastActionID = %q, want ACTION-MODIFY", got)
	}

	// A DELETE removes the active entry entirely, so no LastActionID remains.
	proj.Apply(&CommitEvent{
		ID: "C3", Timestamp: now.Add(2 * time.Minute),
		Changes: []Change{{
			Action: ActionDelete, Path: "a.txt", ID: "ACTION-DELETE",
		}},
	})
	if _, ok := proj.ActiveFiles["a.txt"]; ok {
		t.Errorf("deleted file should not remain active")
	}

	// A MOVE establishes the active entry at the new path with the MOVE's ID.
	proj.Apply(&CommitEvent{
		ID: "C4", Timestamp: now.Add(3 * time.Minute),
		Changes: []Change{{
			Action: ActionMove, Path: "b.txt", OldPath: "c.txt", Hash: "h3", ID: "ACTION-MOVE",
		}},
	})
	if got := proj.ActiveFiles["b.txt"].LastActionID; got != "ACTION-MOVE" {
		t.Errorf("after MOVE, new-path LastActionID = %q, want ACTION-MOVE", got)
	}
}
