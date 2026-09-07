package store

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	// The tool's own state directory and VCS metadata must be excluded even
	// when the user's .gitignore does not mention them.
	stateDir := filepath.Join(workDir, ".snapshots")
	if err := os.MkdirAll(filepath.Join(stateDir, ".internal"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, ".internal", "events.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	gitDir := filepath.Join(workDir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "objects", "packed"), []byte("gitdata"), 0644); err != nil {
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

// TestEngine_NestedGitExclusion ensures a .git directory nested inside the tree
// is pruned too, so a submodule/repo inside the scan root isn't snapshotted.
func TestEngine_NestedGitExclusion(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	os.WriteFile(filepath.Join(workDir, "top.txt"), []byte("top"), 0644)

	// A nested repo checkout under vendor/.
	vendor := filepath.Join(workDir, "vendor")
	nestedGit := filepath.Join(vendor, ".git")
	if err := os.MkdirAll(filepath.Join(nestedGit, "objects"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(nestedGit, "HEAD"), []byte("ref: refs/heads/main"), 0644)
	os.WriteFile(filepath.Join(vendor, "lib.go"), []byte("package vendor"), 0644)

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
	if !paths["top.txt"] {
		t.Errorf("top.txt should be tracked")
	}
	if !paths["vendor/lib.go"] {
		t.Errorf("vendor/lib.go (non-.git content) should be tracked")
	}
	for p := range paths {
		if strings.Contains(p, ".git/") {
			t.Errorf("Path under ignored .git dir was tracked: %q", p)
		}
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

func TestEngine_NewlyIgnoredBecomesIgnored_NotDelete(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	// Track a file.
	filePath := filepath.Join(workDir, "secret.log")
	if err := os.WriteFile(filePath, []byte("sensitive"), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if first == nil || len(first.Changes) != 1 || first.Changes[0].Action != ActionCreate {
		t.Fatalf("Expected a single CREATE for the tracked file, got %+v", first)
	}

	// Introduce an ignore rule that matches the still-present file.
	if err := os.WriteFile(filepath.Join(workDir, ".gitignore"), []byte("*.log\n"), 0644); err != nil {
		t.Fatal(err)
	}

	second, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if second == nil {
		t.Fatalf("Expected changes when an ignore rule is introduced, got nil")
	}

	actions := map[string]ActionType{}
	for _, c := range second.Changes {
		actions[c.Path] = c.Action
	}

	// The file itself must be reported as IGNORED, never DELETE.
	if actions["secret.log"] != ActionIgnored {
		t.Errorf("Expected secret.log to be %s, got %s (changes: %+v)", ActionIgnored, actions["secret.log"], second.Changes)
	}
	// The newly-added .gitignore is itself a tracked CREATE.
	if actions[".gitignore"] != ActionCreate {
		t.Errorf("Expected .gitignore to be a CREATE, got %s", actions[".gitignore"])
	}
	for _, c := range second.Changes {
		if c.Action == ActionDelete {
			t.Errorf("A newly-ignored file must not be emitted as DELETE: %+v", c)
		}
	}

	// Projection: file no longer active, but recorded as ignored (not a tombstone).
	state := engine.State()
	if _, ok := state.ActiveFiles["secret.log"]; ok {
		t.Errorf("secret.log should no longer be an active file")
	}
	if _, ok := state.Ignored["secret.log"]; !ok {
		t.Errorf("secret.log should be recorded as ignored")
	}
	if _, ok := state.Tombstones["secret.log"]; ok {
		t.Errorf("secret.log should NOT have a tombstone (it was not deleted)")
	}

	// No .deleted marker should be written; the content history remains.
	ignoreDir := filepath.Join(ledgerDir, "secret.log")
	entries, err := os.ReadDir(ignoreDir)
	if err != nil {
		t.Fatalf("expected a mirrored dir for secret.log: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".deleted") {
			t.Errorf("unexpected .deleted marker written for an ignored file: %s", e.Name())
		}
	}
	if len(entries) == 0 {
		t.Errorf("expected at least the historical snapshot content to remain for secret.log")
	}
}

func TestEngine_RemovingIgnoreRuleRetracks(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	filePath := filepath.Join(workDir, "cache.tmp")
	if err := os.WriteFile(filePath, []byte("cached"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Snapshot(workDir); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Ignore it.
	if err := os.WriteFile(filepath.Join(workDir, ".gitignore"), []byte("*.tmp\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ign, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if ign == nil {
		t.Fatalf("Expected ignore transition, got nil")
	}

	// Remove the ignore rule -> the file should reappear as a CREATE.
	if err := os.Remove(filepath.Join(workDir, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	third, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if third == nil {
		t.Fatalf("Expected re-tracking when the ignore rule is removed, got nil")
	}
	actions := map[string]ActionType{}
	for _, c := range third.Changes {
		actions[c.Path] = c.Action
	}
	if actions["cache.tmp"] != ActionCreate {
		t.Errorf("Expected cache.tmp to reappear as %s, got %s (changes: %+v)", ActionCreate, actions["cache.tmp"], third.Changes)
	}

	state := engine.State()
	if _, ok := state.ActiveFiles["cache.tmp"]; !ok {
		t.Errorf("cache.tmp should be active again after removing the ignore rule")
	}
	if _, ok := state.Ignored["cache.tmp"]; ok {
		t.Errorf("cache.tmp should no longer be in the ignored set after re-tracking")
	}
}

func TestEngine_TrueDeleteStillDeletes_WhileOtherIgnored(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()

	engine := newTestEngine(t, workDir, ledgerDir)

	keep := filepath.Join(workDir, "secret.log")
	gone := filepath.Join(workDir, "real_delete.txt")
	if err := os.WriteFile(keep, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gone, []byte("will vanish"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Snapshot(workDir); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Ignore the log; physically remove the other file.
	if err := os.WriteFile(filepath.Join(workDir, ".gitignore"), []byte("*.log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	event, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("Expected changes, got nil")
	}
	actions := map[string]ActionType{}
	for _, c := range event.Changes {
		actions[c.Path] = c.Action
	}
	if actions["secret.log"] != ActionIgnored {
		t.Errorf("secret.log should be %s, got %s", ActionIgnored, actions["secret.log"])
	}
	if actions["real_delete.txt"] != ActionDelete {
		t.Errorf("real_delete.txt should still be a true %s, got %s", ActionDelete, actions["real_delete.txt"])
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

// TestEngine_Lineage_CreateModifyDelete verifies that sequential mutations of a
// single file are chained through Change.PreviousID, and that a CREATE starts a
// fresh lineage (no previous_id).
func TestEngine_Lineage_CreateModifyDelete(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()
	engine := newTestEngine(t, workDir, ledgerDir)

	filePath := filepath.Join(workDir, "doc.txt")

	// 1. CREATE starts the lineage root.
	if err := os.WriteFile(filePath, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	create, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}
	if create == nil || len(create.Changes) != 1 {
		t.Fatalf("expected a single CREATE, got %+v", create)
	}
	createCh := create.Changes[0]
	if createCh.Action != ActionCreate {
		t.Fatalf("expected CREATE, got %s", createCh.Action)
	}
	if createCh.ID == "" || len(createCh.ID) != 26 {
		t.Fatalf("expected a 26-char action ID, got %q", createCh.ID)
	}
	if createCh.PreviousID != "" {
		t.Errorf("a CREATE should have no previous_id, got %q", createCh.PreviousID)
	}

	// 2. MODIFY links back to the CREATE's ID.
	if err := os.WriteFile(filePath, []byte("v2_modified"), 0644); err != nil {
		t.Fatal(err)
	}
	mod, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("modify snapshot failed: %v", err)
	}
	if mod == nil || len(mod.Changes) != 1 {
		t.Fatalf("expected a single MODIFY, got %+v", mod)
	}
	modCh := mod.Changes[0]
	if modCh.Action != ActionModify {
		t.Fatalf("expected MODIFY, got %s", modCh.Action)
	}
	if modCh.ID == "" || len(modCh.ID) != 26 {
		t.Errorf("expected a 26-char MODIFY ID, got %q", modCh.ID)
	}
	if modCh.PreviousID != createCh.ID {
		t.Errorf("MODIFY previous_id should reference CREATE id, got %q want %q", modCh.PreviousID, createCh.ID)
	}

	// 3. DELETE links back to the MODIFY's ID.
	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}
	del, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("delete snapshot failed: %v", err)
	}
	if del == nil || len(del.Changes) != 1 {
		t.Fatalf("expected a single DELETE, got %+v", del)
	}
	delCh := del.Changes[0]
	if delCh.Action != ActionDelete {
		t.Fatalf("expected DELETE, got %s", delCh.Action)
	}
	if delCh.PreviousID != modCh.ID {
		t.Errorf("DELETE previous_id should reference MODIFY id, got %q want %q", delCh.PreviousID, modCh.ID)
	}
}

// TestEngine_Lineage_MoveThenModify verifies that a rename (MOVE) carries the
// content lineage across the path change, so a later MODIFY at the new path
// links back to the MOVE action.
func TestEngine_Lineage_MoveThenModify(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()
	engine := newTestEngine(t, workDir, ledgerDir)

	oldPath := filepath.Join(workDir, "old.txt")
	newPath := filepath.Join(workDir, "sub", "new.txt")

	if err := os.WriteFile(oldPath, []byte("same content"), 0644); err != nil {
		t.Fatal(err)
	}
	create, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}
	var createCh *Change
	for i := range create.Changes {
		if create.Changes[i].Action == ActionCreate && create.Changes[i].Path == "old.txt" {
			createCh = &create.Changes[i]
		}
	}
	if createCh == nil {
		t.Fatalf("expected CREATE for old.txt, got %+v", create)
	}

	// Rename to a subdirectory.
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	move, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("move snapshot failed: %v", err)
	}
	var moveCh *Change
	for i := range move.Changes {
		if move.Changes[i].Action == ActionMove && move.Changes[i].OldPath == "old.txt" {
			moveCh = &move.Changes[i]
		}
	}
	if moveCh == nil {
		t.Fatalf("expected MOVE from old.txt, got %+v", move)
	}
	if moveCh.PreviousID != createCh.ID {
		t.Errorf("MOVE previous_id should reference source CREATE id, got %q want %q", moveCh.PreviousID, createCh.ID)
	}

	// Modify content at the new path.
	if err := os.WriteFile(newPath, []byte("changed after move"), 0644); err != nil {
		t.Fatal(err)
	}
	mod, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("modify snapshot failed: %v", err)
	}
	var modCh *Change
	for i := range mod.Changes {
		if mod.Changes[i].Action == ActionModify && mod.Changes[i].Path == "sub/new.txt" {
			modCh = &mod.Changes[i]
		}
	}
	if modCh == nil {
		t.Fatalf("expected MODIFY at new path, got %+v", mod)
	}
	if modCh.PreviousID != moveCh.ID {
		t.Errorf("MODIFY previous_id should reference MOVE id, got %q want %q", modCh.PreviousID, moveCh.ID)
	}
}

// TestEngine_Lineage_IgnoredTransition verifies that a newly-ignored file's
// IGNORED action links back to the last action that produced its active state.
func TestEngine_Lineage_IgnoredTransition(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()
	engine := newTestEngine(t, workDir, ledgerDir)

	filePath := filepath.Join(workDir, "secret.log")
	if err := os.WriteFile(filePath, []byte("sensitive"), 0644); err != nil {
		t.Fatal(err)
	}
	create, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}
	var createCh *Change
	for i := range create.Changes {
		if create.Changes[i].Action == ActionCreate && create.Changes[i].Path == "secret.log" {
			createCh = &create.Changes[i]
		}
	}
	if createCh == nil {
		t.Fatalf("expected CREATE for secret.log, got %+v", create)
	}

	if err := os.WriteFile(filepath.Join(workDir, ".gitignore"), []byte("*.log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ign, err := engine.Snapshot(workDir)
	if err != nil {
		t.Fatalf("ignore snapshot failed: %v", err)
	}
	if ign == nil {
		t.Fatalf("expected an IGNORED transition, got nil")
	}
	var ignCh *Change
	for i := range ign.Changes {
		if ign.Changes[i].Action == ActionIgnored && ign.Changes[i].Path == "secret.log" {
			ignCh = &ign.Changes[i]
		}
	}
	if ignCh == nil {
		t.Fatalf("expected IGNORED for secret.log, got %+v", ign)
	}
	if ignCh.PreviousID != createCh.ID {
		t.Errorf("IGNORED previous_id should reference source CREATE id, got %q want %q", ignCh.PreviousID, createCh.ID)
	}
}

// TestEngine_EqualLengthSameMtime_Recent_Detected reproduces issue 09: a
// tracked file whose bytes change at equal length, with the filesystem reporting
// the *same* modtime (coarse timestamp granularity), must still be reported as a
// MODIFY when the two writes fall inside the fast-path grace window. Before the
// fix, the size+modtime fast path silently dropped such a change.
//
// The snapshot times are injected via snapshotAt so the test is deterministic:
// it does not depend on how fast the host runs between the two writes.
func TestEngine_EqualLengthSameMtime_Recent_Detected(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()
	e, err := NewEngine(workDir, ledgerDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	filePath := filepath.Join(workDir, "data.bin")
	v1 := []byte("AAAA") // len 4
	v2 := []byte("BBBB") // len 4 (equal length, different bytes)
	if len(v1) != len(v2) {
		t.Fatal("test bodies must be equal length")
	}

	// Pin a shared modtime that is comfortably in the past, simulating a coarse
	// filesystem that truncates both writes to the same timestamp quantum.
	t0 := time.Now().UTC().Add(-time.Hour)
	// Second snapshot happens just after the first write's recorded mtime,
	// inside the 1s modTimeGrace window — the case the old fast path dropped.
	tSecond := t0.Add(200 * time.Millisecond)

	if err := os.WriteFile(filePath, v1, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filePath, t0, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := e.snapshotAt(workDir, t0); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}

	// Overwrite with same-length content but keep the identical modtime.
	if err := os.WriteFile(filePath, v2, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filePath, t0, t0); err != nil {
		t.Fatal(err)
	}

	event, err := e.snapshotAt(workDir, tSecond)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if event == nil {
		t.Fatal("equal-length, same-modtime modification was silently dropped (issue 09)")
	}
	mods := 0
	for _, c := range event.Changes {
		if c.Action == ActionModify && c.Path == "data.bin" {
			mods++
		}
	}
	if mods != 1 {
		t.Fatalf("expected a single MODIFY for data.bin, got %+v", event.Changes)
	}
	if got, want := e.State().ActiveFiles["data.bin"].Hash, hashFile(t, filePath); got != want {
		t.Errorf("tracked hash did not advance to the new body: got %s want %s", got, want)
	}
}

// TestEngine_EqualLengthSameMtime_PublicAPI is an end-to-end reproduction of
// issue 09 through the public Snapshot entry point (real wall clock). Two
// equal-length bodies written in rapid succession with a pinned identical mtime
// must still be distinguished by hashing.
func TestEngine_EqualLengthSameMtime_PublicAPI(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()
	e, err := NewEngine(workDir, ledgerDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	filePath := filepath.Join(workDir, "data.bin")
	v1 := []byte("v1xx")
	v2 := []byte("v2yy") // same length, different bytes
	if len(v1) != len(v2) {
		t.Fatal("test bodies must be equal length")
	}

	if err := os.WriteFile(filePath, v1, 0644); err != nil {
		t.Fatal(err)
	}
	first, err := e.Snapshot(workDir)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if first == nil {
		t.Fatal("expected a CREATE on first snapshot")
	}

	// Write the new equal-length body and force the recorded modtime back so the
	// filesystem would report the same mtime as the first write.
	recorded := first.Changes[0].ModTime
	if err := os.WriteFile(filePath, v2, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filePath, recorded, recorded); err != nil {
		t.Fatal(err)
	}

	second, err := e.Snapshot(workDir)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if second == nil {
		t.Fatal("equal-length, same-modtime modification was silently dropped through the public API (issue 09)")
	}
	var sawModify bool
	for _, c := range second.Changes {
		if c.Action == ActionModify && c.Path == "data.bin" {
			sawModify = true
		}
	}
	if !sawModify {
		t.Fatalf("expected a MODIFY for data.bin, got %+v", second.Changes)
	}

	// Re-open the engine from the persisted ledger: the modified content must be
	// the new active state, proving the change was committed, not just noticed.
	reopened, err := NewEngine(workDir, ledgerDir)
	if err != nil {
		t.Fatalf("reopen engine: %v", err)
	}
	if got, want := reopened.State().ActiveFiles["data.bin"].Hash, hashFile(t, filePath); got != want {
		t.Errorf("after reload the active hash should be the new body: got %s want %s", got, want)
	}
}

// TestEngine_UnchangedColdFile_FastPathStillSkips guards AC2: once a tracked
// file's recorded mtime is older than the grace window, an unchanged file must
// still short-circuit (no spurious change emitted) rather than being re-hashed
// and re-reported on every snapshot.
func TestEngine_UnchangedColdFile_FastPathStillSkips(t *testing.T) {
	workDir := t.TempDir()
	ledgerDir := t.TempDir()
	e, err := NewEngine(workDir, ledgerDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	filePath := filepath.Join(workDir, "stable.txt")
	if err := os.WriteFile(filePath, []byte("stable content"), 0644); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now().UTC().Add(-time.Hour)
	if err := os.Chtimes(filePath, t0, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := e.snapshotAt(workDir, t0); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}

	// Far beyond the grace window, an unchanged cold file must produce no change.
	tLater := t0.Add(10 * time.Second)
	event, err := e.snapshotAt(workDir, tLater)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if event != nil {
		t.Fatalf("unchanged cold file should be skipped by the fast path, got %+v", event.Changes)
	}
}
