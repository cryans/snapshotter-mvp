package store

import (
	"os"
	"path/filepath"
	"testing"
)

// newHistoryEngine builds an engine writing its ledger to a separate temp dir,
// mirroring the conventions used elsewhere in this package's tests.
func newHistoryEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	workDir := t.TempDir()
	engine, err := NewEngine(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return engine, workDir
}

// mustSnapshot runs a snapshot and requires that it produced a commit.
func mustSnapshot(t *testing.T, e *Engine, workDir string) *CommitEvent {
	t.Helper()
	event, err := e.Snapshot(workDir)
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if event == nil {
		t.Fatalf("expected a commit, but no changes were detected")
	}
	return event
}

func writeFile(t *testing.T, workDir, rel, content string) {
	t.Helper()
	full := filepath.Join(workDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func deleteFile(t *testing.T, workDir, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(workDir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

func moveFile(t *testing.T, workDir, from, to string) {
	t.Helper()
	if err := os.Rename(
		filepath.Join(workDir, filepath.FromSlash(from)),
		filepath.Join(workDir, filepath.FromSlash(to)),
	); err != nil {
		t.Fatal(err)
	}
}

func historyActions(fh *FileHistory) []string {
	var acts []string
	for _, en := range fh.Entries {
		acts = append(acts, string(en.Action))
	}
	return acts
}

func TestFileHistory_NewestFirst_AcrossMove(t *testing.T) {
	e, wd := newHistoryEngine(t)

	writeFile(t, wd, "a.txt", "a version one content")
	mustSnapshot(t, e, wd) // CREATE a.txt

	writeFile(t, wd, "a.txt", "a version two content is longer")
	mustSnapshot(t, e, wd) // MODIFY a.txt

	moveFile(t, wd, "a.txt", "b.txt")
	mustSnapshot(t, e, wd) // MOVE a.txt -> b.txt

	// The old path: file is gone from disk and moved away, so no (current).
	fh, err := e.FileHistory("a.txt")
	if err != nil {
		t.Fatalf("FileHistory(a.txt) error: %v", err)
	}
	if fh.OnDisk {
		t.Errorf("expected a.txt to be gone from disk")
	}
	if got, want := len(fh.Entries), 3; got != want {
		t.Fatalf("expected %d entries, got %d", want, got)
	}
	// Newest first: MOVE-away, then MODIFY, then CREATE.
	if got := historyActions(fh); !equalStrings(got, []string{"MOVE", "MODIFY", "CREATE"}) {
		t.Errorf("unexpected action order: %v", got)
	}
	mv := fh.Entries[0]
	if mv.Destination != "b.txt" {
		t.Errorf("expected move destination b.txt, got %q", mv.Destination)
	}
	if mv.Current {
		t.Errorf("a.txt is not current; it was moved away")
	}

	// The new path: file is present and active; newest row is the move-into.
	fh2, err := e.FileHistory("b.txt")
	if err != nil {
		t.Fatalf("FileHistory(b.txt) error: %v", err)
	}
	if !fh2.OnDisk {
		t.Errorf("expected b.txt to be on disk")
	}
	if got, want := len(fh2.Entries), 1; got != want {
		t.Fatalf("expected %d entries for b.txt, got %d", want, got)
	}
	in := fh2.Entries[0]
	if in.FromPath != "a.txt" {
		t.Errorf("expected FromPath a.txt, got %q", in.FromPath)
	}
	if !in.Current {
		t.Errorf("b.txt is the current active file; expected Current marker")
	}
}

func TestFileHistory_DeleteAndRecreate_Cycles(t *testing.T) {
	e, wd := newHistoryEngine(t)

	writeFile(t, wd, "x.txt", "one")
	mustSnapshot(t, e, wd) // CREATE x.txt

	deleteFile(t, wd, "x.txt")
	mustSnapshot(t, e, wd) // DELETE x.txt

	// After deletion the file is not current.
	fh, err := e.FileHistory("x.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := historyActions(fh), []string{"DELETE", "CREATE"}; !equalStrings(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
	if fh.Entries[0].Current {
		t.Errorf("deleted file should not be current")
	}

	// Recreate a different file at the same path -> both cycles present,
	// newest CREATE is current.
	writeFile(t, wd, "x.txt", "two")
	mustSnapshot(t, e, wd) // CREATE x.txt (fresh lineage)

	fh2, err := e.FileHistory("x.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !fh2.OnDisk {
		t.Errorf("recreated x.txt should be on disk")
	}
	if got, want := historyActions(fh2), []string{"CREATE", "DELETE", "CREATE"}; !equalStrings(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
	if !fh2.Entries[0].Current {
		t.Errorf("recreated x.txt should be current")
	}
}

func TestFileHistory_OnlyTouchingPathsAreListed(t *testing.T) {
	e, wd := newHistoryEngine(t)

	writeFile(t, wd, "a.txt", "aaa")
	writeFile(t, wd, "b.txt", "bbb")
	mustSnapshot(t, e, wd)

	fh, err := e.FileHistory("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(fh.Entries), 1; got != want {
		t.Fatalf("expected %d entry for a.txt, got %d", want, got)
	}
	if fh.Entries[0].Action != ActionCreate {
		t.Errorf("expected CREATE, got %s", fh.Entries[0].Action)
	}
}

func TestFileHistory_OnDiskButNeverSnapshotted(t *testing.T) {
	e, wd := newHistoryEngine(t)
	writeFile(t, wd, "newfile.txt", "hello")

	fh, err := e.FileHistory("newfile.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !fh.OnDisk {
		t.Errorf("expected newfile.txt to be on disk")
	}
	if len(fh.Entries) != 0 {
		t.Errorf("expected no history entries, got %d", len(fh.Entries))
	}
}

func TestFileHistory_NotOnDiskNoHistory(t *testing.T) {
	e, wd := newHistoryEngine(t)
	_ = wd

	fh, err := e.FileHistory("ghost.txt")
	if err != nil {
		t.Fatal(err)
	}
	if fh.OnDisk {
		t.Errorf("expected ghost.txt not to be on disk")
	}
	if len(fh.Entries) != 0 {
		t.Errorf("expected no history entries for ghost.txt")
	}
}

func TestFileHistory_ResolveTargetRejectsBadArgs(t *testing.T) {
	e, _ := newHistoryEngine(t)

	for _, arg := range []string{"", ".", "../escape.txt", ".snapshots/x", ".git/config"} {
		if _, err := e.FileHistory(arg); err == nil {
			t.Errorf("expected error for target %q", arg)
		}
	}
}

func TestFileHistory_AbsolutePathWithinWorkspace(t *testing.T) {
	e, wd := newHistoryEngine(t)
	writeFile(t, wd, "sub/f.txt", "content")
	mustSnapshot(t, e, wd)

	abs := filepath.Join(wd, "sub", "f.txt")
	fh, err := e.FileHistory(abs)
	if err != nil {
		t.Fatalf("absolute path within workspace should resolve: %v", err)
	}
	if fh.Path != "sub/f.txt" {
		t.Errorf("expected normalized path sub/f.txt, got %q", fh.Path)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
