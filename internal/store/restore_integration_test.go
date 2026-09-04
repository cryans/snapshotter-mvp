package store

// Integration tests for single-file restores (issue 04). They drive the engine
// exactly as the CLI does (NewEngine(workDir, workDir/.snapshots) with the
// mirror inside the scanned tree), snapshot a short history, then call
// Engine.Restore and assert on the reconstructed work-tree bytes and the
// conflict-backup area. A tiny sleep separates consecutive commits so their
// millisecond timestamps never collide in the mirrored blob names.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// settle separates commits so mirror blob timestamps are distinct.
func settle() { time.Sleep(2 * time.Millisecond) }

func readWorkFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read work file %q: %v", rel, err)
	}
	return string(b)
}

// conflictDir returns the run-time conflict backup root under the mirror.
func conflictDir(root string) string {
	return filepath.Join(root, ".snapshots", ".internal", "conflicts")
}

// readFirstBackup walks the conflicts tree and returns the first backup file's
// content plus its path, or ("", "", false) if no backup exists.
func readFirstBackup(t *testing.T, root, rel string) (content, path string, ok bool) {
	t.Helper()
	rootc := conflictDir(root)
	_ = filepath.Join(rootc, filepath.FromSlash(rel))
	// Walk the whole conflicts area to find a file whose suffix matches rel.
	var found string
	err := filepath.Walk(rootc, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(filepath.ToSlash(p), rel) {
			found = p
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false
		}
		t.Fatalf("walk conflicts: %v", err)
	}
	if found == "" {
		return "", "", false
	}
	b, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("read backup %q: %v", found, err)
	}
	return string(b), found, true
}

// --- Scenario: reverting a modified file to an earlier commit ----------------

func TestIntegration_RestoreModifiedFile(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "a.txt", "version one")
	_, first := snapshotActions(t, e, root)
	if first == nil {
		t.Fatal("expected an initial CREATE commit")
	}

	settle()
	writeRel(t, root, "a.txt", "version two with a longer body")
	_, second := snapshotActions(t, e, root)
	if second == nil {
		t.Fatal("expected a MODIFY commit")
	}

	// Restore a.txt to the content it held at the first commit.
	res, err := e.Restore(first.ID, "a.txt")
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !res.Restored {
		t.Fatal("expected the file to be rewritten on restore")
	}
	if got := readWorkFile(t, root, "a.txt"); got != "version one" {
		t.Errorf("restored content = %q, want %q", got, "version one")
	}

	// The divergent current content was preserved, not silently dropped.
	if content, _, ok := readFirstBackup(t, root, "a.txt"); !ok || content != "version two with a longer body" {
		t.Errorf("expected divergent content backed up to conflicts, got content=%q ok=%v", content, ok)
	}
	if res.BackupPath == "" {
		t.Error("RestoreResult.BackupPath should name the conflict backup location")
	}
}

// --- Scenario: recreating a file that was deleted after an earlier commit -----

func TestIntegration_RestoreDeletedFile(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "b.txt", "keep me")
	_, created := snapshotActions(t, e, root)
	if created == nil {
		t.Fatal("expected an initial CREATE commit")
	}

	settle()
	rmRel(t, root, "b.txt")
	_, deleted := snapshotActions(t, e, root)
	if deleted == nil {
		t.Fatal("expected a DELETE commit")
	}

	// The file no longer exists on disk.
	if _, err := os.Stat(filepath.Join(root, "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("b.txt should have been deleted before restore")
	}

	// Restore it back to the commit where it was still active.
	res, err := e.Restore(created.ID, "b.txt")
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !res.Restored {
		t.Fatal("expected a deleted file to be recreated on restore")
	}
	if got := readWorkFile(t, root, "b.txt"); got != "keep me" {
		t.Errorf("restored content = %q, want %q", got, "keep me")
	}
	// Nothing diverged, so no conflict backup was made.
	if _, _, ok := readFirstBackup(t, root, "b.txt"); ok {
		t.Error("did not expect a conflict backup when the file was simply missing")
	}
}

// --- Scenario: restore is idempotent when content already matches -------------

func TestIntegration_RestoreIdempotentWhenAlreadyCurrent(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "c.txt", "stable")
	_, first := snapshotActions(t, e, root)
	if first == nil {
		t.Fatal("expected an initial CREATE commit")
	}

	// The work tree already matches the commit; restore is a no-op.
	res, err := e.Restore(first.ID, "c.txt")
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if res.Restored {
		t.Error("restore should be a no-op when content already matches")
	}
	if res.BackupPath != "" {
		t.Errorf("no conflict backup expected on a no-op restore, got %q", res.BackupPath)
	}
	if _, _, ok := readFirstBackup(t, root, "c.txt"); ok {
		t.Error("no conflict backup should exist on a no-op restore")
	}
}

// --- Scenario: single-target restore leaves unrelated files untouched ---------

func TestIntegration_RestoreIsSingleTarget(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "d.txt", "d one")
	writeRel(t, root, "e.txt", "e one")
	_, first := snapshotActions(t, e, root)
	if first == nil {
		t.Fatal("expected an initial CREATE commit")
	}

	settle()
	writeRel(t, root, "d.txt", "d two longer")
	writeRel(t, root, "e.txt", "e two longer")
	_, second := snapshotActions(t, e, root)
	if second == nil {
		t.Fatal("expected a MODIFY commit")
	}

	// Restore only d.txt; e.txt must remain at its (newer) content.
	if _, err := e.Restore(first.ID, "d.txt"); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if got := readWorkFile(t, root, "d.txt"); got != "d one" {
		t.Errorf("d.txt = %q, want %q", got, "d one")
	}
	if got := readWorkFile(t, root, "e.txt"); got != "e two longer" {
		t.Errorf("e.txt was modified by a single-target restore: got %q, want %q", got, "e two longer")
	}
}

// --- Scenario: errors for an unknown commit and a path absent at that commit --

func TestIntegration_RestoreErrors(t *testing.T) {
	e, root := newIntegrationEngine(t)

	writeRel(t, root, "f.txt", "f one")
	_, first := snapshotActions(t, e, root)
	if first == nil {
		t.Fatal("expected an initial CREATE commit")
	}

	if _, err := e.Restore("00MISSINGCOMMIT000000000000", "f.txt"); err == nil {
		t.Error("expected an error restoring from an unknown commit")
	}

	// A path that never existed at the commit must not be restorable.
	if _, err := e.Restore(first.ID, "never-tracked.txt"); err == nil {
		t.Error("expected an error restoring a path that was not active at the commit")
	}

	// Refuse to touch the tool's own state dir.
	if _, err := e.Restore(first.ID, ".snapshots"); err == nil {
		t.Error("expected an error when targeting the tool's own state dir")
	}
}
