package store

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	// A .gitignore that ignores common build artifacts and caches. The engine
	// must follow these rules rather than auto-ignoring arbitrary dot-entries.
	gitignore := []byte(".go-cache/\n.scratch/\n*.log\nsnapshotter_bin\nbin/\n")
	if err := os.WriteFile(filepath.Join(workDir, ".gitignore"), gitignore, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a normal tracking file.
	if err := os.WriteFile(filepath.Join(workDir, "tracked.txt"), []byte("tracked"), 0644); err != nil {
		t.Fatal(err)
	}

	// Dot directories and build artifacts that .gitignore rules exclude.
	for _, dirName := range []string{".scratch", ".go-cache", "bin"} {
		dirPath := filepath.Join(workDir, dirName)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "should_be_ignored.txt"), []byte("ignored"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Ignored files at the root.
	for _, fileName := range []string{"debug.log", "snapshotter_bin"} {
		if err := os.WriteFile(filepath.Join(workDir, fileName), []byte("ignored file"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// This should NOT be auto-ignored: our design does not blanket-exclude every
	// dot-entry, only what the .gitignore rules say plus the hard .snapshots rule.
	dotTracked := filepath.Join(workDir, ".tracked-dotfile")
	if err := os.WriteFile(dotTracked, []byte("dot tracked"), 0644); err != nil {
		t.Fatal(err)
	}

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event with changes, got nil")
	}

	got := make(map[string]bool)
	for _, c := range event.Changes {
		got[c.Path] = true
	}

	for path := range got {
		if path != "tracked.txt" && path != ".gitignore" && path != ".tracked-dotfile" {
			t.Errorf("Unexpected tracked path: %q", path)
		}
	}

	for _, required := range []string{"tracked.txt", ".gitignore", ".tracked-dotfile"} {
		if !got[required] {
			t.Errorf("Expected %q to be tracked, but it was not", required)
		}
	}
}

func TestEngine_HardcodedSnapshotsExclusion(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	// The tool's own state directory must be excluded even when the user's
	// .gitignore does not mention it.
	stateDir := filepath.Join(workDir, ".snapshots")
	if err := os.MkdirAll(filepath.Join(stateDir, ".internal"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, ".internal", "events.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// No .gitignore present, so nothing else is excluded.
	if err := os.WriteFile(filepath.Join(workDir, "tracked.txt"), []byte("tracked"), 0644); err != nil {
		t.Fatal(err)
	}

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event with changes, got nil")
	}

	if len(event.Changes) != 1 {
		t.Fatalf("Expected exactly 1 tracked file, got %d: %+v", len(event.Changes), event.Changes)
	}
	if event.Changes[0].Path != "tracked.txt" {
		t.Errorf("Expected only tracked.txt, got %q", event.Changes[0].Path)
	}
}

func TestEngine_GitIgnoreNegation(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	// Ignore all logs, then re-include important.log within the same file.
	gi := []byte("*.log\n!important.log\n")
	if err := os.WriteFile(filepath.Join(workDir, ".gitignore"), gi, 0644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(workDir, "debug.log"), []byte("debug"), 0644)
	os.WriteFile(filepath.Join(workDir, "important.log"), []byte("keep me"), 0644)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event, got nil")
	}

	paths := map[string]bool{}
	for _, c := range event.Changes {
		paths[c.Path] = true
	}
	if paths["debug.log"] {
		t.Errorf("debug.log should be ignored")
	}
	if !paths["important.log"] {
		t.Errorf("important.log should be re-included via negation")
	}
	if !paths[".gitignore"] {
		t.Errorf(".gitignore should be tracked")
	}
}

func TestEngine_GitIgnoreDirectoryOnly(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	// Directory-only pattern (trailing slash) must prune the whole subtree.
	gi := []byte("build/\n")
	if err := os.WriteFile(filepath.Join(workDir, ".gitignore"), gi, 0644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(workDir, "keep.txt"), []byte("keep"), 0644)
	deep := filepath.Join(workDir, "build", "out", "artifact.bin")
	if err := os.MkdirAll(filepath.Dir(deep), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(deep, []byte("binary"), 0644)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event, got nil")
	}

	paths := map[string]bool{}
	for _, c := range event.Changes {
		paths[c.Path] = true
	}
	if !paths["keep.txt"] {
		t.Errorf("keep.txt should be tracked")
	}
	for p := range paths {
		if len(p) >= 5 && p[:6] == "build/" {
			t.Errorf("Path under ignored build/ dir was tracked: %q", p)
		}
	}
}

func TestEngine_NestedGitIgnore(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	// Root .gitignore ignores nothing by default; a nested subproject applies its
	// own additional rules scoped to its directory.
	subDir := filepath.Join(workDir, "subproject")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	nestedGi := []byte("*.tmp\nlocal-secret/\n")
	if err := os.WriteFile(filepath.Join(subDir, ".gitignore"), nestedGi, 0644); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(subDir, "main.go"), []byte("package sub"), 0644)
	os.WriteFile(filepath.Join(subDir, "scratch.tmp"), []byte("temp"), 0644)
	if err := os.MkdirAll(filepath.Join(subDir, "local-secret"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(subDir, "local-secret", "key.pem"), []byte("secret"), 0644)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event, got nil")
	}

	paths := map[string]bool{}
	for _, c := range event.Changes {
		paths[c.Path] = true
	}
	if !paths["subproject/main.go"] {
		t.Errorf("subproject/main.go should be tracked")
	}
	if paths["subproject/scratch.tmp"] {
		t.Errorf("subproject/scratch.tmp should be ignored by nested rule")
	}
	for p := range paths {
		if len(p) >= len("subproject/local-secret/") && p[:len("subproject/local-secret/")] == "subproject/local-secret/" {
			t.Errorf("Path under ignored nested dir was tracked: %q", p)
		}
	}
	if !paths["subproject/.gitignore"] {
		t.Errorf("nested .gitignore should be tracked")
	}
}

func TestEngine_ForwardSlashPaths(t *testing.T) {
	// Paths emitted by the engine must always use forward slashes so the ledger
	// stays platform-independent, even on Windows where os.PathSeparator is '\\'.
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	deep := filepath.Join(workDir, "a", "b", "c.txt")
	if err := os.MkdirAll(filepath.Dir(deep), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(deep, []byte("nested"), 0644)

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event, got nil")
	}
	for _, c := range event.Changes {
		for _, r := range c.Path {
			if r == filepath.Separator && r != '/' {
				t.Errorf("Path %q contains a native path separator", c.Path)
			}
		}
		if !strings.Contains(c.Path, "/") {
			t.Errorf("Expected forward slashes in path, got %q", c.Path)
		}
	}
}
