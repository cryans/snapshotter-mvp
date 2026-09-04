package store

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type SnapshotEngine interface {
	Snapshot(dir string) (*CommitEvent, error)
	State() *Projection
}

func hashFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file for hashing: %v", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("failed to hash file: %v", err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func newTestEngine(t *testing.T, workDir, ledgerDir string) SnapshotEngine {
	t.Helper()
	engine, err := NewEngine(workDir, ledgerDir)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return engine
}

func TestEngine_Create(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	filePath := filepath.Join(workDir, "hello.txt")
	content := []byte("hello, world!")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event to not be nil (no changes detected)")
	}

	if len(event.Changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(event.Changes))
	}
	change := event.Changes[0]
	if change.Action != ActionCreate {
		t.Errorf("Expected action %s, got %s", ActionCreate, change.Action)
	}
	if change.Path != "hello.txt" {
		t.Errorf("Expected path 'hello.txt', got %q", change.Path)
	}

	state := engine.State()
	fileState, ok := state.ActiveFiles["hello.txt"]
	if !ok {
		t.Fatalf("Expected 'hello.txt' in active files")
	}
	
	expectedHash := hashFile(t, filePath)
	if fileState.Hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, fileState.Hash)
	}
}

func TestEngine_Update(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	filePath := filepath.Join(workDir, "data.txt")
	os.WriteFile(filePath, []byte("v1"), 0644)
	
	engine.Snapshot(workDir)

	os.WriteFile(filePath, []byte("v2_updated"), 0644)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event to not be nil (no changes detected)")
	}

	if len(event.Changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(event.Changes))
	}
	change := event.Changes[0]
	if change.Action != ActionModify {
		t.Errorf("Expected action %s, got %s", ActionModify, change.Action)
	}

	state := engine.State()
	expectedHash := hashFile(t, filePath)
	if state.ActiveFiles["data.txt"].Hash != expectedHash {
		t.Errorf("State hash did not update")
	}
}

func TestEngine_Delete(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	filePath := filepath.Join(workDir, "temp.txt")
	os.WriteFile(filePath, []byte("ephemeral"), 0644)
	engine.Snapshot(workDir)

	os.Remove(filePath)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event to not be nil (no changes detected)")
	}

	if len(event.Changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(event.Changes))
	}
	if event.Changes[0].Action != ActionDelete {
		t.Errorf("Expected action %s, got %s", ActionDelete, event.Changes[0].Action)
	}

	state := engine.State()
	if _, ok := state.ActiveFiles["temp.txt"]; ok {
		t.Errorf("Expected 'temp.txt' to be removed from active files")
	}
	if _, ok := state.Tombstones["temp.txt"]; !ok {
		t.Errorf("Expected 'temp.txt' to be present in tombstones")
	}
}

func TestEngine_Move(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	oldPath := filepath.Join(workDir, "old.txt")
	newPath := filepath.Join(workDir, "new.txt")
	content := []byte("move me")
	
	os.WriteFile(oldPath, content, 0644)
	engine.Snapshot(workDir)

	os.Rename(oldPath, newPath)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event to not be nil (no changes detected)")
	}

	if len(event.Changes) != 1 {
		t.Fatalf("Expected 1 change for move, got %d", len(event.Changes))
	}
	change := event.Changes[0]
	if change.Action != ActionMove {
		t.Errorf("Expected action %s, got %s", ActionMove, change.Action)
	}
	if change.OldPath != "old.txt" {
		t.Errorf("Expected OldPath 'old.txt', got %q", change.OldPath)
	}
	if change.Path != "new.txt" {
		t.Errorf("Expected Path 'new.txt', got %q", change.Path)
	}

	state := engine.State()
	if _, ok := state.ActiveFiles["old.txt"]; ok {
		t.Errorf("Expected 'old.txt' to be removed from active files")
	}
	if _, ok := state.ActiveFiles["new.txt"]; !ok {
		t.Errorf("Expected 'new.txt' to be in active files")
	}
}

func TestEngine_Subdirectories(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	subDir := filepath.Join(workDir, "docs", "specs")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	filePath := filepath.Join(subDir, "arch.md")
	os.WriteFile(filePath, []byte("architecture v1"), 0644)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event to not be nil (no changes detected)")
	}

	if len(event.Changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(event.Changes))
	}
	change := event.Changes[0]
	if change.Action != ActionCreate {
		t.Errorf("Expected action %s, got %s", ActionCreate, change.Action)
	}
	
	expectedRel := filepath.ToSlash(filepath.Join("docs", "specs", "arch.md"))
	if filepath.ToSlash(change.Path) != expectedRel {
		t.Errorf("Expected path %q, got %q", expectedRel, change.Path)
	}

	newDir := filepath.Join(workDir, "archive")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatal(err)
	}
	newFilePath := filepath.Join(newDir, "arch.md")
	os.Rename(filePath, newFilePath)

	event2, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event2 == nil {
		t.Fatalf("Expected event to not be nil (no changes detected)")
	}

	if len(event2.Changes) != 1 {
		t.Fatalf("Expected 1 move change, got %d", len(event2.Changes))
	}
	change2 := event2.Changes[0]
	if change2.Action != ActionMove {
		t.Errorf("Expected action %s, got %s", ActionMove, change2.Action)
	}

	expectedOld := expectedRel
	expectedNew := filepath.ToSlash(filepath.Join("archive", "arch.md"))
	
	if filepath.ToSlash(change2.OldPath) != expectedOld {
		t.Errorf("Expected OldPath %q, got %q", expectedOld, change2.OldPath)
	}
	if filepath.ToSlash(change2.Path) != expectedNew {
		t.Errorf("Expected Path %q, got %q", expectedNew, change2.Path)
	}
}

func TestEngine_Exclusions(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	// Create normal tracking file
	if err := os.WriteFile(filepath.Join(workDir, "tracked.txt"), []byte("tracked"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create excluded directories
	ignoredDirs := []string{".snapshots", ".scratch", ".git", ".go-cache", ".any-dot-dir"}
	for _, dirName := range ignoredDirs {
		dirPath := filepath.Join(workDir, dirName)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatal(err)
		}
		// Create file inside ignored directory
		if err := os.WriteFile(filepath.Join(dirPath, "should_be_ignored.txt"), []byte("ignored"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create ignored files at the root
	ignoredFiles := []string{".hidden_file", "snapshotter_bin"}
	for _, fileName := range ignoredFiles {
		if err := os.WriteFile(filepath.Join(workDir, fileName), []byte("ignored file"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Snapshot
	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event with 1 change, got nil")
	}

	// We should only have 1 change corresponding to tracked.txt
	if len(event.Changes) != 1 {
		t.Fatalf("Expected exactly 1 tracked file change, got %d: %+v", len(event.Changes), event.Changes)
	}

	if event.Changes[0].Path != "tracked.txt" {
		t.Errorf("Expected only tracked.txt to be snapshot, but got: %s", event.Changes[0].Path)
	}
}
