package store

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// walker performs a physical directory walk while applying git-style
// `.gitignore` rules. Every directory that contains a `.gitignore` contributes
// its own "frame" whose patterns are interpreted relative to that directory,
// mirroring how git scopes patterns. The walk always descends from the scan
// root downward, pruning any directory (and its subtree) that a rule excludes,
// which naturally prevents a deeper `.gitignore` from re-including a path whose
// parent directory was already excluded.
type walker struct {
	root string // absolute path of the scan root
}

// ignoreFrame is a single `.gitignore` file bound to the directory it lives in.
type ignoreFrame struct {
	dirRel string // slash-separated directory path relative to the scan root ("" = root)
	gi     *gitignore.GitIgnore
}

// trackedFile is a file that passed all ignore rules and should be snapshotted.
type trackedFile struct {
	rel string // slash-separated path relative to the scan root
	abs string // absolute path on disk
}

// newWalker initializes a walker over the given scan root.
func newWalker(root string) *walker {
	return &walker{root: root}
}

// collect walks the scan root and returns every trackable file.
func (w *walker) collect() ([]trackedFile, error) {
	var out []trackedFile
	// No ignore frames apply to the scan root itself.
	if err := w.walkDir(w.root, "", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// walkDir recurses into one directory. `relDir` is the directory's path relative
// to the scan root ("" for the root itself). `frames` holds the active ignore
// rules for this directory's *contents* (including this directory's own
// `.gitignore`, if any). Trackable files found are appended to `out`.
func (w *walker) walkDir(absDir, relDir string, frames []ignoreFrame, out *[]trackedFile) error {
	// Load this directory's own .gitignore and scope it to this directory, so its
	// rules govern everything directly inside it (and its descendants).
	childFrames := frames
	if gi, ok := w.loadGitIgnore(absDir); ok {
		childFrames = append(childFrames, ignoreFrame{dirRel: relDir, gi: gi})
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		isDir := entry.IsDir()

		// Hard-coded exclusion: the tool's internal state directory is never
		// snapshotted, regardless of any .gitignore rules.
		if name == ".snapshots" {
			continue
		}

		rel := relDir
		if rel != "" {
			rel += "/"
		}
		rel += name

		if w.isIgnored(rel, isDir, childFrames) {
			continue
		}

		if isDir {
			childAbs := filepath.Join(absDir, name)
			if err := w.walkDir(childAbs, rel, childFrames, out); err != nil {
				return err
			}
			continue
		}

		// Only regular files are snapshotted. This avoids following symlinks and
		// reparse points, which could otherwise introduce cycles or duplicate data.
		if !entry.Type().IsRegular() {
			continue
		}

		*out = append(*out, trackedFile{
			rel: rel,
			abs: filepath.Join(absDir, name),
		})
	}
	return nil
}

// loadGitIgnore compiles a directory's .gitignore, returning (nil, false) when
// the file is absent (which is the common case and not an error).
func (w *walker) loadGitIgnore(dir string) (*gitignore.GitIgnore, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil, false
	}
	lines := strings.Split(string(raw), "\n")
	return gitignore.CompileIgnoreLines(lines...), true
}

// isIgnored reports whether a path (file or directory) is excluded by the given
// ignore frames. Git-style semantics apply: rules are evaluated from the scan
// root down to the directory containing the target, and the deepest frame that
// produces a definitive decision wins. Directories are matched with a trailing
// slash so directory-only patterns like "build/" still prune them.
func (w *walker) isIgnored(rel string, isDir bool, frames []ignoreFrame) bool {
	matchPath := rel
	if isDir {
		matchPath += "/"
	}

	decision := false
	for _, frame := range frames {
		relToFrame := matchPath
		if frame.dirRel != "" {
			relToFrame = strings.TrimPrefix(matchPath, frame.dirRel+"/")
		}
		ignored, pattern := frame.gi.MatchesPathHow(relToFrame)
		// pattern == nil means this frame produced no match at all, so it has no
		// opinion and must not override a shallower decision.
		if pattern != nil {
			decision = ignored
		}
	}
	return decision
}
