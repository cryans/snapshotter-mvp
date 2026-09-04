package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"snapshotter/internal/store"
)

func ts(h, m int) time.Time {
	return time.Date(2026, 9, 4, h, m, 0, 0, time.UTC)
}

func TestBuildRows_LabelsAndCurrent(t *testing.T) {
	fh := &store.FileHistory{
		Path:   "x.txt",
		OnDisk: true,
		Entries: []store.HistoryEntry{
			{CommitID: "c3", Timestamp: ts(3, 0), Action: store.ActionCreate, Current: true},
			{CommitID: "c2", Timestamp: ts(2, 0), Action: store.ActionDelete},
			{CommitID: "c1", Timestamp: ts(1, 0), Action: store.ActionCreate},
		},
	}

	rows := buildRows(fh)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// Newest first -> CREATE (current), DELETE (deleted), CREATE.
	if rows[0].commit != "c3" || rows[0].current != true || !strings.Contains(rows[0].labels, "(current)") {
		t.Errorf("row 0 should be the current create: %+v", rows[0])
	}
	if rows[1].commit != "c2" || rows[1].labels != "(deleted)" {
		t.Errorf("row 1 should be deleted: %+v", rows[1])
	}
	if rows[2].commit != "c1" || rows[2].labels != "" {
		t.Errorf("row 2 should be an unlabelled create: %+v", rows[2])
	}
}

func TestBuildRows_MoveAwayAndMoveIn(t *testing.T) {
	fh := &store.FileHistory{
		Path: "a.txt",
		Entries: []store.HistoryEntry{
			// a.txt moved away to b.txt (newest).
			{CommitID: "c2", Timestamp: ts(2, 0), Action: store.ActionMove, Destination: "b.txt"},
			// a.txt arrived from old.txt earlier.
			{CommitID: "c1", Timestamp: ts(1, 0), Action: store.ActionMove, FromPath: "old.txt"},
		},
	}
	rows := buildRows(fh)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	away := rows[0]
	if away.dest != "b.txt" || !strings.Contains(away.desc, "b.txt") || away.labels != "(moved)" {
		t.Errorf("move-away row wrong: %+v", away)
	}
	in := rows[1]
	if in.dest != "" || !strings.Contains(in.desc, "old.txt") {
		t.Errorf("move-in row wrong: %+v", in)
	}
}

func TestHistoryModel_EnterFollowsMove(t *testing.T) {
	e, wd := newTUEngine(t)

	writeFileForTUI(t, wd, "a.txt", "content one long enough")
	snap(t, e, wd) // CREATE a.txt

	moveForTUI(t, wd, "a.txt", "b.txt")
	snap(t, e, wd) // MOVE a.txt -> b.txt

	model, err := newHistoryModel(e, wd, "a.txt")
	if err != nil {
		t.Fatalf("newHistoryModel(a.txt): %v", err)
	}
	if model.path != "a.txt" || len(model.rows) == 0 {
		t.Fatalf("expected a.txt history rows, path=%q rows=%d", model.path, len(model.rows))
	}
	// Newest row is the move-away; press Enter on it.
	if model.rows[0].dest != "b.txt" {
		t.Fatalf("expected newest row to follow to b.txt, got dest=%q", model.rows[0].dest)
	}

	got, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("unexpected command from Enter: %v", cmd)
	}
	next, ok := got.(*historyModel)
	if !ok {
		t.Fatalf("expected *historyModel, got %T", got)
	}
	if next.path != "b.txt" {
		t.Errorf("expected view to navigate to b.txt, got %q", next.path)
	}
	if len(next.rows) != 1 {
		t.Errorf("expected b.txt to have 1 row, got %d", len(next.rows))
	}
}

func TestHistoryModel_Quit(t *testing.T) {
	fh := &store.FileHistory{Path: "x.txt", OnDisk: true}
	m2 := &historyModel{path: "x.txt"}
	m2.rows = buildRows(fh)
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Errorf("expected a quit command on 'q'")
	}
}

func TestBuildRows_RestorableKinds(t *testing.T) {
	fh := &store.FileHistory{
		Path: "f.txt",
		Entries: []store.HistoryEntry{
			{CommitID: "c1", Timestamp: ts(1, 0), Action: store.ActionCreate},
			{CommitID: "c2", Timestamp: ts(2, 0), Action: store.ActionModify},
			{CommitID: "c3", Timestamp: ts(3, 0), Action: store.ActionDelete},
			{CommitID: "c4", Timestamp: ts(4, 0), Action: store.ActionIgnored},
			{CommitID: "c5", Timestamp: ts(5, 0), Action: store.ActionMove, Destination: "g.txt"},
			{CommitID: "c6", Timestamp: ts(6, 0), Action: store.ActionMove, FromPath: "h.txt"},
		},
	}
	rows := buildRows(fh)
	want := map[string]bool{"c1": true, "c2": true, "c3": false, "c4": false, "c5": false, "c6": true}
	byCommit := map[string]viewRow{}
	for _, r := range rows {
		byCommit[r.commit] = r
	}
	for commit, restorable := range want {
		r, ok := byCommit[commit]
		if !ok {
			t.Fatalf("missing row for %s", commit)
		}
		if r.restorable != restorable {
			t.Errorf("row %s restorable = %v, want %v", commit, r.restorable, restorable)
		}
	}
	// A move-away row is followable but not restorable.
	if away := byCommit["c5"]; away.dest != "g.txt" || away.restorable {
		t.Errorf("move-away row should follow but not restore: %+v", away)
	}
}

func TestHistoryModel_EnterRestoresVersionAndRecordsEntry(t *testing.T) {
	e, wd := newTUEngine(t)

	v1 := "restore content version one"
	v2 := "restore content version two is longer"
	writeFileForTUI(t, wd, "f.txt", v1)
	snap(t, e, wd)                   // CREATE f.txt (v1)
	time.Sleep(3 * time.Millisecond) // keep mirror blob timestamps distinct
	writeFileForTUI(t, wd, "f.txt", v2)
	snap(t, e, wd)                   // MODIFY f.txt (v2, current)
	time.Sleep(3 * time.Millisecond) // so the restore's follow-up snapshot gets a fresh timestamp

	model, err := newHistoryModel(e, wd, "f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(model.rows) != 2 || !model.rows[0].current {
		t.Fatalf("expected [MODIFY(current), CREATE], got %d rows (top current=%v)", len(model.rows), model.rows[0].current)
	}

	// Move the cursor down to the CREATE (v1) row and press Enter to restore v1.
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	got, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("unexpected command: %v", cmd)
	}
	m := got.(*historyModel)

	// Disk content should now be v1...
	disk, err := os.ReadFile(filepath.Join(wd, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != v1 {
		t.Errorf("expected restored content %q, got %q", v1, disk)
	}
	// ...and a new (current) entry should have been recorded and shown.
	if len(m.rows) != 3 {
		t.Fatalf("expected 3 rows after restore, got %d", len(m.rows))
	}
	if !m.rows[0].current {
		t.Errorf("expected the new entry to be current")
	}
	if !strings.Contains(m.notice, "Restored f.txt") {
		t.Errorf("expected a restore notice, got %q", m.notice)
	}
}

func TestHistoryModel_EnterOnCurrentIsNoop(t *testing.T) {
	e, wd := newTUEngine(t)

	writeFileForTUI(t, wd, "f.txt", "some content v1 length")
	snap(t, e, wd) // CREATE

	model, err := newHistoryModel(e, wd, "f.txt")
	if err != nil {
		t.Fatal(err)
	}
	// cursor is on the (current) row.
	got, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := got.(*historyModel)
	if len(m.rows) != 1 {
		t.Fatalf("expected rows unchanged, got %d", len(m.rows))
	}
	if !strings.Contains(m.notice, "current version") {
		t.Errorf("expected a 'current version' notice, got %q", m.notice)
	}
}

// --- helpers (main package tests) ---

func newTUEngine(t *testing.T) (*store.Engine, string) {
	t.Helper()
	wd := t.TempDir()
	e, err := store.NewEngine(wd, t.TempDir())
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return e, wd
}

func writeFileForTUI(t *testing.T, wd, rel, content string) {
	t.Helper()
	full := filepath.Join(wd, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func moveForTUI(t *testing.T, wd, from, to string) {
	t.Helper()
	if err := os.Rename(
		filepath.Join(wd, filepath.FromSlash(from)),
		filepath.Join(wd, filepath.FromSlash(to)),
	); err != nil {
		t.Fatal(err)
	}
}

func snap(t *testing.T, e *store.Engine, wd string) {
	t.Helper()
	ev, err := e.Snapshot(wd)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected a commit")
	}
}
