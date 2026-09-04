package store

// End-to-end integration tests for the snapshot engine (issue 07).
//
// Unlike the unit tests in engine_test.go (which point the engine at a throwaway
// ledger dir separate from the work tree), these drive the engine exactly the
// way the real CLI does: NewEngine(workDir, workDir/.snapshots) with the state
// mirror living INSIDE the scanned tree. Each test seeds its own throwaway tree
// via t.TempDir(), replays a scenario from the manual /tmp/snap-demo walk-through,
// and asserts on the (action, path) change sets plus the on-disk .snapshots mirror.
// Empty directories are never snapshotted, so only file-level changes matter.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// markerSuffixes are the ledger marker file extensions appended after a path's
// real extension. A content snapshot never carries one; a marker always does.
var markerSuffixes = []string{".deleted", ".ignored", ".moved", ".moved_from"}

func isMarkerName(name string) bool {
	for _, m := range markerSuffixes {
		if strings.HasSuffix(name, m) {
			return true
		}
	}
	return false
}

// newIntegrationEngine creates a fresh scratch tree whose state mirror lives at
// <root>/.snapshots, matching how the CLI runs the tool against a real directory.
func newIntegrationEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	root := t.TempDir()
	eng, err := NewEngine(root, filepath.Join(root, ".snapshots"))
	if err != nil {
		t.Fatalf("failed to create integration engine: %v", err)
	}
	return eng, root
}

// writeRel writes a file at a slash-separated relative path, creating parents.
func writeRel(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %q: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", rel, err)
	}
}

// mkdirRel creates an empty directory at a slash-separated relative path.
func mkdirRel(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", rel, err)
	}
}

// rmRel removes a file at a slash-separated relative path.
func rmRel(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("remove %q: %v", rel, err)
	}
}

// renameRel renames oldRel -> newRel (slash-separated), creating new parents.
func renameRel(t *testing.T, root, oldRel, newRel string) {
	t.Helper()
	src := filepath.Join(root, filepath.FromSlash(oldRel))
	dst := filepath.Join(root, filepath.FromSlash(newRel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir for %q: %v", newRel, err)
	}
	if err := os.Rename(src, dst); err != nil {
		t.Fatalf("rename %q -> %q: %v", oldRel, newRel, err)
	}
}

// snapshotActions runs one snapshot and returns the (path -> action) map. The
// returned commit is nil when there were no changes.
func snapshotActions(t *testing.T, e *Engine, root string) (map[string]ActionType, *CommitEvent) {
	t.Helper()
	ev, err := e.Snapshot(root)
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	m := map[string]ActionType{}
	if ev != nil {
		for _, c := range ev.Changes {
			m[c.Path] = c.Action
		}
	}
	return m, ev
}

// assertActionsEqual asserts the observed (path -> action) set equals want.
func assertActionsEqual(t *testing.T, want, got map[string]ActionType) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("change count = %d, want %d (got %#v)", len(got), len(want), got)
	}
	for p, a := range want {
		if ga, ok := got[p]; !ok {
			t.Errorf("missing change for %q", p)
		} else if ga != a {
			t.Errorf("action for %q = %s, want %s", p, ga, a)
		}
	}
	for p, a := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("unexpected change: %q -> %s", p, a)
		}
	}
}

// mirrorAbs returns the on-disk mirror directory for a slash-separated path.
func mirrorAbs(root, rel string) string {
	return filepath.Join(root, ".snapshots", filepath.FromSlash(rel))
}

// readMirrorContent returns the most recent non-marker content snapshot for a
// tracked file's mirror directory.
func readMirrorContent(t *testing.T, root, rel string) string {
	t.Helper()
	dir := mirrorAbs(root, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read mirror dir for %q: %v", rel, err)
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if isMarkerName(entries[i].Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, entries[i].Name()))
		if err != nil {
			t.Fatalf("read mirror content for %q: %v", rel, err)
		}
		return string(b)
	}
	t.Fatalf("no non-marker content snapshot found for %q in %s", rel, dir)
	return ""
}

// findMirrorSuffixFile returns the contents of the single mirror file whose name
// ends in suffix (e.g. ".ignored" or ".moved"), or ("", false) if absent.
func findMirrorSuffixFile(t *testing.T, root, rel, suffix string) (string, bool) {
	t.Helper()
	entries, err := os.ReadDir(mirrorAbs(root, rel))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false
		}
		t.Fatalf("read mirror dir for %q: %v", rel, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			b, err := os.ReadFile(filepath.Join(mirrorAbs(root, rel), e.Name()))
			if err != nil {
				t.Fatalf("read %s marker for %q: %v", suffix, rel, err)
			}
			return string(b), true
		}
	}
	return "", false
}

// assertNoMirrorMarkers asserts none of the given marker suffixes are present in
// a path's mirror directory (used to prove an ignored file isn't written as a
// .deleted tombstone marker).
func assertNoMirrorMarkers(t *testing.T, root, rel string, suffixes ...string) {
	t.Helper()
	entries, err := os.ReadDir(mirrorAbs(root, rel))
	if err != nil {
		t.Fatalf("read mirror dir for %q: %v", rel, err)
	}
	bySuffix := map[string]bool{}
	for _, s := range suffixes {
		bySuffix[s] = true
	}
	for _, e := range entries {
		for s := range bySuffix {
			if strings.HasSuffix(e.Name(), s) {
				t.Errorf("unexpected %s marker written for %q: %s", s, rel, e.Name())
			}
		}
	}
}

// assertMirrorHasMarker asserts exactly that a marker with suffix exists and,
// when wantContent is non-empty, that its bytes match.
func assertMirrorHasMarker(t *testing.T, root, rel, suffix, wantContent string) {
	t.Helper()
	content, ok := findMirrorSuffixFile(t, root, rel, suffix)
	if !ok {
		t.Errorf("expected a %s marker in mirror dir for %q", suffix, rel)
		return
	}
	if wantContent != "" && content != wantContent {
		t.Errorf("%s marker content for %q = %q, want %q", suffix, rel, content, wantContent)
	}
}

// ledgerLineCount returns how many commits were appended to events.jsonl.
func ledgerLineCount(t *testing.T, root string) int {
	t.Helper()
	p := filepath.Join(root, ".snapshots", ".internal", "events.jsonl")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(b) == 0 {
		return 0
	}
	return strings.Count(string(b), "\n")
}

// --- Scenario: initial snapshot of a fresh tree produces CREATEs + mirror ------

func TestIntegration_InitialSnapshotCreatesAndMirrors(t *testing.T) {
	e, root := newIntegrationEngine(t)

	seed := map[string]string{
		"readme.md":         "hello world",
		"secret.log":        "secret data",
		"sub/cache.tmp":     "cachey",
		"sub/deep/note.txt": "nested",
		"build/out.bin":     "binary-ish",
	}
	for rel, content := range seed {
		writeRel(t, root, rel, content)
	}

	actions, ev := snapshotActions(t, e, root)
	if ev == nil {
		t.Fatal("expected a CREATE commit on the first snapshot of a fresh tree")
	}
	assertActionsEqual(t, map[string]ActionType{
		"readme.md":         ActionCreate,
		"secret.log":        ActionCreate,
		"sub/cache.tmp":     ActionCreate,
		"sub/deep/note.txt": ActionCreate,
		"build/out.bin":     ActionCreate,
	}, actions)

	// Each tracked file's bytes are mirrored under .snapshots/<path>/.
	for rel, content := range seed {
		if got := readMirrorContent(t, root, rel); got != content {
			t.Errorf("mirrored content for %q = %q, want %q", rel, got, content)
		}
	}

	// A second, unchanged snapshot yields no commit ("No changes detected.").
	if _, again := snapshotActions(t, e, root); again != nil {
		t.Error("expected nil event on an unchanged second snapshot")
	}

	// The append-only ledger carries exactly one commit line.
	if n := ledgerLineCount(t, root); n != 1 {
		t.Errorf("ledger line count = %d, want 1", n)
	}
}

// --- Scenario: hard-coded .git / .snapshots exclusions (incl. nested) ----------

func TestIntegration_HardcodedExclusionsNeverSnapshotted(t *testing.T) {
	e, root := newIntegrationEngine(t)

	// Real tracked content.
	writeRel(t, root, "top.txt", "top")

	// Root-level .git plumbing must never be tracked.
	mkdirRel(t, root, ".git/objects")
	writeRel(t, root, ".git/HEAD", "ref: refs/heads/main")
	writeRel(t, root, ".git/objects/pack", "gitdata")

	// A nested repo under vendor/: .git pruned, real content kept.
	writeRel(t, root, "vendor/lib.go", "package vendor")
	mkdirRel(t, root, "vendor/.git/objects")
	writeRel(t, root, "vendor/.git/HEAD", "ref: refs/heads/main")

	// A stray .snapshots dir nested in the tree must be pruned too.
	writeRel(t, root, "nested/keep.txt", "keep")
	writeRel(t, root, "nested/.snapshots/leak.txt", "must not track")

	actions, ev := snapshotActions(t, e, root)
	if ev == nil {
		t.Fatal("expected a commit with tracked content")
	}

	want := map[string]ActionType{
		"top.txt":         ActionCreate,
		"vendor/lib.go":   ActionCreate,
		"nested/keep.txt": ActionCreate,
	}
	assertActionsEqual(t, want, actions)

	// The engine never snapshots its own state even though the mirror + ledger
	// now live inside the scanned tree: a follow-up snapshot is a no-op.
	if _, again := snapshotActions(t, e, root); again != nil {
		t.Error("engine re-tracked its own .snapshots state on a follow-up snapshot")
	}
}

// --- Scenario: introducing an ignore rule flips present files to IGNORED ------

func TestIntegration_NewlyIgnoredFlipsToIgnored_NotDelete(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "readme.md", "hello world")
	writeRel(t, root, "secret.log", "secret data")
	writeRel(t, root, "sub/cache.tmp", "cachey")
	writeRel(t, root, "sub/deep/note.txt", "nested")

	if _, first := snapshotActions(t, e, root); first == nil {
		t.Fatal("expected an initial CREATE commit")
	}

	// Introduce ignore rules matching still-present files (mirrors demo step).
	writeRel(t, root, ".gitignore", "*.log\n*.tmp\nbuild/\n")

	actions, ev := snapshotActions(t, e, root)
	if ev == nil {
		t.Fatal("expected changes when an ignore rule is introduced")
	}

	// .gitignore itself is newly tracked; matched files flip to IGNORED, never DELETE.
	assertActionsEqual(t, map[string]ActionType{
		".gitignore":    ActionCreate,
		"secret.log":    ActionIgnored,
		"sub/cache.tmp": ActionIgnored,
	}, actions)

	// Mirrored markers: an .ignored marker exists while the original content
	// history survives, and crucially no .deleted tombstone is written.
	for _, rel := range []string{"secret.log", "sub/cache.tmp"} {
		assertMirrorHasMarker(t, root, rel, ".ignored", "")
		if content := readMirrorContent(t, root, rel); content == "" {
			t.Errorf("expected preserved content history for ignored %q", rel)
		}
		assertNoMirrorMarkers(t, root, rel, ".deleted")
	}

	// Projection: files are ignored (not tombstoned) and no longer active.
	state := e.State()
	for _, rel := range []string{"secret.log", "sub/cache.tmp"} {
		if _, ok := state.ActiveFiles[rel]; ok {
			t.Errorf("%q should no longer be active after being ignored", rel)
		}
		if _, ok := state.Ignored[rel]; !ok {
			t.Errorf("%q should be recorded as ignored", rel)
		}
		if _, ok := state.Tombstones[rel]; ok {
			t.Errorf("%q must not have a tombstone (it was ignored, not deleted)", rel)
		}
	}

	// --- Removing the ignore rule re-tracks the files as CREATE (issue 06 lifecycle).
	rmRel(t, root, ".gitignore")
	actions2, ev2 := snapshotActions(t, e, root)
	if ev2 == nil {
		t.Fatal("expected changes when the ignore rule is removed")
	}
	assertActionsEqual(t, map[string]ActionType{
		".gitignore":    ActionDelete,
		"secret.log":    ActionCreate,
		"sub/cache.tmp": ActionCreate,
	}, actions2)

	state = e.State()
	for _, rel := range []string{"secret.log", "sub/cache.tmp"} {
		if _, ok := state.ActiveFiles[rel]; !ok {
			t.Errorf("%q should be active again after the ignore rule is removed", rel)
		}
		if _, ok := state.Ignored[rel]; ok {
			t.Errorf("%q should no longer be ignored after re-tracking", rel)
		}
	}
}

// --- Scenario: .gitignore negation (!pattern) re-includes a matched path ------

func TestIntegration_IgnoreNegationReincludes(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, ".gitignore", "*.log\n!important.log\n")
	writeRel(t, root, "readme.md", "hello")
	writeRel(t, root, "debug.log", "debug")
	writeRel(t, root, "important.log", "keep me")

	actions, ev := snapshotActions(t, e, root)
	if ev == nil {
		t.Fatal("expected a commit")
	}

	// debug.log is excluded, but important.log is re-included via negation.
	assertActionsEqual(t, map[string]ActionType{
		".gitignore":    ActionCreate,
		"readme.md":     ActionCreate,
		"important.log": ActionCreate,
	}, actions)
	if _, tracked := actions["debug.log"]; tracked {
		t.Error("debug.log should be ignored despite negation of a sibling")
	}
}

// --- Scenario: nested .gitignore rules apply only within their own scope ------

func TestIntegration_NestedGitIgnoreScoped(t *testing.T) {
	e, root := newIntegrationEngine(t)

	// Root has no .gitignore; only subproject/ carries its own *.tmp rule.
	writeRel(t, root, "readme.md", "hello")
	writeRel(t, root, "top.tmp", "root tmp — should NOT be ignored")
	writeRel(t, root, "subproject/main.go", "package sub")
	writeRel(t, root, "subproject/scratch.tmp", "temp")
	writeRel(t, root, "subproject/.gitignore", "*.tmp\n")

	actions, ev := snapshotActions(t, e, root)
	if ev == nil {
		t.Fatal("expected a commit")
	}

	assertActionsEqual(t, map[string]ActionType{
		"readme.md":             ActionCreate,
		"top.tmp":               ActionCreate, // root file, outside nested scope
		"subproject/.gitignore": ActionCreate,
		"subproject/main.go":    ActionCreate,
	}, actions)
	if _, tracked := actions["subproject/scratch.tmp"]; tracked {
		t.Error("subproject/scratch.tmp should be ignored by the nested rule")
	}
}

// --- Scenario: content move detected as a single MOVE --------------------------

func TestIntegration_ContentMoveDetected(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "sub/cache.tmp", "cachey")
	if _, first := snapshotActions(t, e, root); first == nil {
		t.Fatal("expected an initial CREATE commit")
	}

	// Rename the tracked file: content matches the tracked-but-missing source.
	renameRel(t, root, "sub/cache.tmp", "sub/cache2.tmp")

	actions, ev := snapshotActions(t, e, root)
	if ev == nil {
		t.Fatal("expected a MOVE commit")
	}
	if len(ev.Changes) != 1 {
		t.Fatalf("expected a single MOVE, got %d changes: %+v", len(ev.Changes), ev.Changes)
	}
	c := ev.Changes[0]
	if c.Action != ActionMove {
		t.Errorf("expected MOVE, got %s", c.Action)
	}
	if c.OldPath != "sub/cache.tmp" {
		t.Errorf("OldPath = %q, want %q", c.OldPath, "sub/cache.tmp")
	}
	if c.Path != "sub/cache2.tmp" {
		t.Errorf("Path = %q, want %q", c.Path, "sub/cache2.tmp")
	}
	if _, ok := actions["sub/cache.tmp"]; ok {
		t.Error("move must not also emit a DELETE/CREATE for the source path")
	}
	// The single event is keyed by its destination path and is a MOVE, not CREATE.
	if a := actions["sub/cache2.tmp"]; a != ActionMove {
		t.Errorf("destination should be recorded as MOVE, got %s", a)
	}

	// Mirror markers: .moved at the old path (pointing to new), .moved_from at
	// the new path (pointing to old) plus copied content.
	assertMirrorHasMarker(t, root, "sub/cache.tmp", ".moved", "sub/cache2.tmp")
	assertMirrorHasMarker(t, root, "sub/cache2.tmp", ".moved_from", "sub/cache.tmp")
	if got := readMirrorContent(t, root, "sub/cache2.tmp"); got != "cachey" {
		t.Errorf("mirrored destination content = %q, want %q", got, "cachey")
	}

	// Projection: tracked at the new path, source no longer active.
	state := e.State()
	if _, ok := state.ActiveFiles["sub/cache.tmp"]; ok {
		t.Error("sub/cache.tmp should not be active after the move")
	}
	if _, ok := state.ActiveFiles["sub/cache2.tmp"]; !ok {
		t.Error("sub/cache2.tmp should be active after the move")
	}
}

// --- Scenario: every ledger/change path uses forward slashes ------------------

func TestIntegration_ForwardSlashPaths(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "a/b/c/d.txt", "deep")

	actions, ev := snapshotActions(t, e, root)
	if ev == nil {
		t.Fatal("expected a commit")
	}
	for _, c := range ev.Changes {
		for _, p := range []string{c.Path, c.OldPath} {
			if p == "" {
				continue
			}
			if strings.ContainsRune(p, '\\') {
				t.Errorf("path %q contains a backslash; ledger paths must use forward slashes", p)
			}
			if filepath.ToSlash(p) != p {
				t.Errorf("path %q is not slash-normalized", p)
			}
		}
	}
	if _, ok := actions["a/b/c/d.txt"]; !ok {
		t.Error("expected the nested path to be tracked with forward slashes")
	}

	// The serialized ledger must carry slash-separated paths.
	b, err := os.ReadFile(filepath.Join(root, ".snapshots", ".internal", "events.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if !strings.Contains(string(b), `"path":"a/b/c/d.txt"`) {
		t.Errorf("ledger does not serialize the path with forward slashes: %s", b)
	}
}
