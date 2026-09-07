package store

// Direct unit tests for the restore blob-lookup helpers (issue 11 coverage
// follow-up). Unlike the integration tests, these call findContentBlob and
// isRestoreMarker on synthetic mirror directories so the defensive fallback and
// marker-exclusion branches are exercised deterministically.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeMirrorFile lays down a file inside a synthetic .snapshots/<rel> directory.
func writeMirrorFile(t *testing.T, root, rel, name string) string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir mirror dir %q: %v", rel, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
		t.Fatalf("write mirror file %q: %v", name, err)
	}
	return p
}

func TestIsRestoreMarker(t *testing.T) {
	for _, name := range []string{
		"2026-09-07T12-00-00.000.txt.deleted",
		"2026-09-07T12-00-00.000.txt.ignored",
		"2026-09-07T12-00-00.000.txt.moved",
		"2026-09-07T12-00-00.000.txt.moved_from",
	} {
		if !isRestoreMarker(name) {
			t.Errorf("isRestoreMarker(%q) = false, want true", name)
		}
	}

	for _, name := range []string{
		// A genuine content blob (exact name) must never be misread as a marker.
		"2026-09-07T12-00-00.000.txt",
		// A content name that merely shares a marker substring is still content.
		"2026-09-07T12-00-00.000.moved.txt",
		"notes.txt",
		"",
	} {
		if isRestoreMarker(name) {
			t.Errorf("isRestoreMarker(%q) = true, want false", name)
		}
	}
}

// TestFindContentBlob_ExactMatch verifies the fast path returns the blob whose
// name exactly equals <writeTS><ext>, even when other entries surround it.
func TestFindContentBlob_ExactMatch(t *testing.T) {
	root := t.TempDir()
	// Markers around the exact candidate must not confuse the exact-name lookup.
	writeMirrorFile(t, root, "notes.txt", "2026-09-07T11-00-00.000.txt.deleted")
	exact := writeMirrorFile(t, root, "notes.txt", "2026-09-07T12-00-00.000.txt")
	writeMirrorFile(t, root, "notes.txt", "2026-09-07T13-00-00.000.txt")

	if got := findContentBlob(root, "notes.txt", "2026-09-07T12-00-00.000"); got != exact {
		t.Errorf("findContentBlob = %q, want %q", got, exact)
	}
}

// TestFindContentBlob_LegacyFallback drives the defensive path taken when the
// exact-name candidate is absent (an older commit predating millisecond
// precision): the newest non-marker content snapshot at-or-before writeTS wins.
func TestFindContentBlob_LegacyFallback(t *testing.T) {
	root := t.TempDir()
	writeMirrorFile(t, root, "notes.txt", "2026-09-07T10-00-00.000.txt")
	// A newer marker shares the millisecond window but must never be treated as
	// content (isRestoreMarker exclusion).
	writeMirrorFile(t, root, "notes.txt", "2026-09-07T12-00-00.000.txt.moved_from")
	// The newest content at-or-before the (nonexistent) exact writeTS.
	writeMirrorFile(t, root, "notes.txt", "2026-09-07T11-30-00.000.txt")

	want := filepath.Join(root, "notes.txt", "2026-09-07T11-30-00.000.txt")
	if got := findContentBlob(root, "notes.txt", "2026-09-07T12-00-00.000"); got != want {
		t.Errorf("fallback findContentBlob = %q, want %q", got, want)
	}
}

// TestFindContentBlob_LegacyFallback_NoLaterContent ensures a marker dated after
// writeTS is not selected even if it is the only file present.
func TestFindContentBlob_LegacyFallback_NoLaterContent(t *testing.T) {
	root := t.TempDir()
	// Only a marker exists, newer than writeTS -> no content blob.
	writeMirrorFile(t, root, "notes.txt", "2026-09-07T12-00-00.000.txt.deleted")
	if got := findContentBlob(root, "notes.txt", "2026-09-07T11-00-00.000"); got != "" {
		t.Errorf("findContentBlob = %q, want empty (only a marker present)", got)
	}
}

// TestFindContentBlob_MissingDir returns empty when the mirror dir is absent.
func TestFindContentBlob_MissingDir(t *testing.T) {
	root := t.TempDir()
	if got := findContentBlob(root, "does/not/exist.txt", "2026-09-07T12-00-00.000"); got != "" {
		t.Errorf("findContentBlob = %q, want empty for a missing mirror dir", got)
	}
}
