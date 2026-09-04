package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"snapshotter/internal/store"
)

// runHistory implements `snapshotter <filename>`: it resolves the target and
// launches the interactive history viewer. A file that exists in neither the
// active workspace nor the ledger is a clean error (issue 03).
func runHistory(engine *store.Engine, workDir string, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: snapshotter <filename>")
		os.Exit(2)
	}

	fh, err := engine.FileHistory(args[0])
	if err != nil {
		log.Fatalf("Failed to read history: %v", err)
	}

	// Acceptance criterion: clean error when the file is absent from both the
	// active workspace and the ledger.
	if !fh.OnDisk && len(fh.Entries) == 0 {
		fmt.Fprintf(os.Stderr, "snapshotter: %q does not exist in the workspace or in the snapshot history.\n", fh.Path)
		os.Exit(1)
	}

	model, err := newHistoryModel(engine, workDir, fh.Path)
	if err != nil {
		log.Fatalf("Failed to open history view: %v", err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}

// viewRow is one renderable line of a path's history.
type viewRow struct {
	when       string // formatted timestamp
	commit     string // commit ULID
	desc       string // e.g. "created", "modified", "moved to src/new.go"
	labels     string // e.g. "(current)", "(deleted)", "(moved)", possibly combined
	dest       string // non-empty when Enter follows this row to a new location
	current    bool   // true when this is the newest entry and the file is active now
	restorable bool   // true when Enter restores this row's content (a content version)
}

// historyModel is the bubbletea model driving the interactive file-history
// viewer for a single path. It owns the rendered rows and the keyboard cursor.
type historyModel struct {
	engine  *store.Engine
	workDir string // the workspace to snapshot after a restore
	path    string // the path currently being viewed
	rows    []viewRow
	cursor  int
	offset  int // scroll offset into rows for the visible window
	width   int
	height  int
	notice  string // transient message (e.g. a failed follow/restore)
}

// newHistoryModel resolves arg against the engine and builds the initial model
// for that path. It returns a descriptive error only when the target is
// structurally invalid (the caller decides not-found messaging).
func newHistoryModel(engine *store.Engine, workDir, arg string) (*historyModel, error) {
	fh, err := engine.FileHistory(arg)
	if err != nil {
		return nil, err
	}
	m := &historyModel{engine: engine, workDir: workDir, path: fh.Path}
	m.rows = buildRows(fh)
	return m, nil
}

// buildRows turns a FileHistory into display rows, newest first.
func buildRows(fh *store.FileHistory) []viewRow {
	rows := make([]viewRow, 0, len(fh.Entries))
	for _, en := range fh.Entries {
		when := en.Timestamp.UTC().Format("2006-01-02 15:04:05")
		r := viewRow{when: when, commit: en.CommitID, current: en.Current}

		switch en.Action {
		case store.ActionCreate:
			r.desc = "created"
			r.restorable = true
		case store.ActionModify:
			r.desc = "modified"
			r.restorable = true
		case store.ActionDelete:
			r.desc = "deleted"
			r.labels = "(deleted)"
		case store.ActionIgnored:
			r.desc = "ignored"
			r.labels = "(ignored)"
		case store.ActionMove:
			switch {
			case en.Destination != "": // moved away -> followable
				r.desc = "moved to " + en.Destination
				r.dest = en.Destination
				r.labels = "(moved)"
			case en.FromPath != "": // moved into this path -> restorable content
				r.desc = "moved from " + en.FromPath
				r.restorable = true
			default:
				r.desc = "moved"
			}
		}

		if en.Current {
			if r.labels == "" {
				r.labels = "(current)"
			} else {
				r.labels += " (current)"
			}
		}
		rows = append(rows, r)
	}
	return rows
}

// Init implements tea.Model.
func (m *historyModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *historyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.rows) == 0 {
				return m, nil
			}
			r := m.rows[m.cursor]
			switch {
			case r.dest != "": // a (moved) row -> follow to the new location
				m.follow(r.dest)
			case r.restorable && !r.current: // a historical content version -> restore it
				m.restore(r)
			case r.current:
				m.notice = fmt.Sprintf("%s is already at the current version.", m.path)
			default:
				m.notice = "Nothing to restore on this entry (deleted/ignored)."
			}
		}
		m.clampScroll()
	}
	return m, nil
}

// follow navigates the viewer to the history of a new destination path.
func (m *historyModel) follow(dest string) {
	next, err := newHistoryModel(m.engine, m.workDir, dest)
	if err != nil {
		m.notice = fmt.Sprintf("cannot open %q: %v", dest, err)
		return
	}
	*m = *next
	m.notice = fmt.Sprintf("Following move → %s", m.path)
}

// restore writes the highlighted version's content back to the currently
// displayed file name, then immediately snapshots so the restored state is
// recorded as a new history entry. The view is refreshed with that new entry as
// the (current) top row. Restoring under a name that differs from the latest
// version is safe: any divergent live content is backed up by the engine before
// being overwritten.
func (m *historyModel) restore(r viewRow) {
	res, err := m.engine.Restore(r.commit, m.path)
	if err != nil {
		m.notice = fmt.Sprintf("restore failed: %v", err)
		return
	}
	if !res.Restored {
		m.notice = fmt.Sprintf("%s is already at this version; no change recorded.", m.path)
		return
	}

	backupNote := ""
	if res.BackupPath != "" {
		backupNote = fmt.Sprintf("; divergent content backed up to %s", res.BackupPath)
	}

	// Record the restored content as a fresh history entry immediately.
	if _, err := m.engine.Snapshot(m.workDir); err != nil {
		m.notice = fmt.Sprintf("restored %s but the follow-up snapshot failed: %v", m.path, err)
		return
	}
	if err := m.reload(); err != nil {
		m.notice = fmt.Sprintf("restored %s but the view could not refresh: %v", m.path, err)
		return
	}
	m.notice = fmt.Sprintf("Restored %s to commit %s; new snapshot recorded%s.", m.path, r.commit, backupNote)
}

// reload re-reads the current path's history from the ledger and resets the
// cursor to the top so the new (current) entry is immediately visible.
func (m *historyModel) reload() error {
	fh, err := m.engine.FileHistory(m.path)
	if err != nil {
		return err
	}
	m.rows = buildRows(fh)
	m.cursor = 0
	m.offset = 0
	m.clampScroll()
	return nil
}

// View implements tea.Model.
func (m *historyModel) View() string {
	if m.height == 0 {
		m.height = 24
	}
	if m.width == 0 {
		m.width = 80
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render("History: " + m.path)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString("  No recorded history for this path yet.\n")
		if m.notice != "" {
			b.WriteString("\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(m.notice) + "\n")
		}
		b.WriteString("\n  Press q to quit.\n")
		return b.String()
	}

	// Visible window.
	avail := m.height - 4 // title + spacing + footer
	if avail < 1 {
		avail = 1
	}
	end := m.offset + avail
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.offset; i < end; i++ {
		r := m.rows[i]
		line := fmt.Sprintf("%s  %s  %s  %s", r.when, r.commit, r.desc, r.labels)
		line = clipToWidth(line, m.width-2)
		if i == m.cursor {
			line = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("236")).
				Render("▶ " + line)
			b.WriteString(line)
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	if m.notice != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(m.notice) + "\n")
	}

	b.WriteString("\n")
	footer := lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("%d of %d entries   ↑/↓ or j/k to move · Enter restores (follows a move) · q quits", m.cursor+1, len(m.rows)))
	b.WriteString(footer)
	return b.String()
}

// clipToWidth truncates s to at most max runes so a row never spills past the
// terminal edge or forces lipgloss to wrap it across lines.
func clipToWidth(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// clampScroll keeps the cursor within the visible window.
func (m *historyModel) clampScroll() {
	if m.height <= 0 {
		return
	}
	avail := m.height - 4
	if avail < 1 {
		avail = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+avail {
		m.offset = m.cursor - avail + 1
	}
	if m.offset > len(m.rows)-avail {
		if m.offset < 0 {
			m.offset = 0
		}
	}
	if len(m.rows) <= avail {
		m.offset = 0
	}
}
