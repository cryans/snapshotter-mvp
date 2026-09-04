package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLedger_Append(t *testing.T) {
	workDir := t.TempDir()
	snapDir := t.TempDir()

	ledger := NewLedger(workDir, snapDir)

	// Setup a dummy file in the work dir to copy
	os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("v1"), 0644)
	os.MkdirAll(filepath.Join(workDir, "nested"), 0755)
	os.WriteFile(filepath.Join(workDir, "nested/docs.md"), []byte("docs v1"), 0644)

	now := time.Date(2026, 9, 3, 12, 2, 15, 510000000, time.UTC)
	commit := &CommitEvent{
		ID:        NewCommitID(now),
		Timestamp: now,
		Message:   "Initial commit",
		Changes: []Change{
			{Action: ActionCreate, Path: "hello.txt"},
			{Action: ActionModify, Path: "nested/docs.md"},
			{Action: ActionDelete, Path: "old.txt"},
			{Action: ActionMove, Path: "new.txt", OldPath: "renamed.txt"},
		},
	}
	
	// Create the moved file in work dir so we can read it
	os.WriteFile(filepath.Join(workDir, "new.txt"), []byte("moved content"), 0644)

	err := ledger.Append(commit)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// 1. Verify events.jsonl
	eventsPath := filepath.Join(snapDir, ".internal", "events.jsonl")
	b, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("Failed to read events.jsonl: %v", err)
	}
	
	var readCommit CommitEvent
	if err := json.Unmarshal(b, &readCommit); err != nil {
		t.Fatalf("Failed to parse events.jsonl: %v", err)
	}
	if readCommit.ID != commit.ID {
		t.Errorf("Expected commit ID %s, got %s", commit.ID, readCommit.ID)
	}

	// 2. Verify Mirrored tree
	ts := "2026-09-03T12-02-15.510"

	// CREATE
	helloContent, _ := os.ReadFile(filepath.Join(snapDir, "hello.txt", ts+".txt"))
	if !bytes.Equal(helloContent, []byte("v1")) {
		t.Errorf("Expected hello.txt content 'v1', got %q", helloContent)
	}

	// MODIFY (nested)
	docsContent, _ := os.ReadFile(filepath.Join(snapDir, "nested/docs.md", ts+".md"))
	if !bytes.Equal(docsContent, []byte("docs v1")) {
		t.Errorf("Expected nested/docs.md content 'docs v1', got %q", docsContent)
	}

	// DELETE
	delInfo, err := os.Stat(filepath.Join(snapDir, "old.txt", ts+".txt.deleted"))
	if err != nil {
		t.Errorf("Missing deleted marker for old.txt: %v", err)
	} else if delInfo.Size() != 0 {
		t.Errorf("Expected deleted marker to be empty, got size %d", delInfo.Size())
	}

	// MOVE (.moved)
	movedMarker, _ := os.ReadFile(filepath.Join(snapDir, "renamed.txt", ts+".txt.moved"))
	if !bytes.Equal(movedMarker, []byte("new.txt")) {
		t.Errorf("Expected moved marker to point to 'new.txt', got %q", movedMarker)
	}

	// MOVE (.moved_from)
	movedFromMarker, _ := os.ReadFile(filepath.Join(snapDir, "new.txt", ts+".txt.moved_from"))
	if !bytes.Equal(movedFromMarker, []byte("renamed.txt")) {
		t.Errorf("Expected moved_from marker to point to 'renamed.txt', got %q", movedFromMarker)
	}

	// MOVE (actual content)
	movedContent, _ := os.ReadFile(filepath.Join(snapDir, "new.txt", ts+".txt"))
	if !bytes.Equal(movedContent, []byte("moved content")) {
		t.Errorf("Expected moved content 'moved content', got %q", movedContent)
	}
}

func TestLedger_Append_Multiple(t *testing.T) {
	workDir := t.TempDir()
	snapDir := t.TempDir()

	ledger := NewLedger(workDir, snapDir)
	
	for i := 0; i < 3; i++ {
		now := time.Now()
		c := &CommitEvent{
			ID:        NewCommitID(now),
			Timestamp: now,
			Message:   "commit",
		}
		if err := ledger.Append(c); err != nil {
			t.Fatal(err)
		}
	}

	f, err := os.Open(filepath.Join(snapDir, ".internal", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	
	if count != 3 {
		t.Errorf("Expected 3 events in jsonl, got %d", count)
	}
}
