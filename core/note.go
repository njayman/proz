package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Note is a project's left-off note: one line per signal, in a fixed order,
// composed from signals so the user never writes it.
type Note struct {
	Lines []string
}

const shortNoteLen = 120

// Full is the whole note, one line per signal, for email and the CLI.
func (n Note) Full() string { return strings.Join(n.Lines, "\n") }

// Short is the note on one line, truncated to shortNoteLen runes, for
// notifications and the tray.
func (n Note) Short() string {
	s := []rune(strings.Join(n.Lines, "; "))
	if len(s) <= shortNoteLen {
		return string(s)
	}
	return string(s[:shortNoteLen-3]) + "..."
}

func quietLine(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return plural("Quiet for %d day", int(d/(24*time.Hour)))
	case d >= time.Hour:
		return plural("Quiet for %d hour", int(d/time.Hour))
	default:
		return "Quiet for less than an hour"
	}
}

func plural(format string, n int) string {
	s := fmt.Sprintf(format, n)
	if n != 1 {
		s += "s"
	}
	return s
}

// ProjectAt returns the project whose folder is dir or the nearest folder
// above it, so agents and the CLI can work from anywhere inside a project.
func (s *Store) ProjectAt(dir string) (Project, error) {
	dir = resolve(dir)
	projects, err := s.Projects()
	if err != nil {
		return Project{}, err
	}
	var best Project
	bestLen := -1
	for _, p := range projects {
		if p.Path == "" {
			continue
		}
		root := resolve(p.Path)
		if len(root) <= bestLen {
			continue
		}
		if within(dir, root) {
			best, bestLen = p, len(root)
		}
	}
	if bestLen < 0 {
		return Project{}, ErrNotFound
	}
	return best, nil
}

// within reports whether dir is root or inside it; both already resolved.
func within(dir, root string) bool {
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return dir == root || strings.HasPrefix(dir, prefix)
}

// resolve cleans path and follows symlinks, so /tmp and /private/tmp (macOS)
// or a symlinked home compare equal. A path that cannot be resolved, such as
// a deleted folder, is compared as written.
// ponytail: case-sensitive compare; case-insensitive filesystems (macOS, Windows) need os.SameFile on the ancestors if users hit it.
func resolve(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return filepath.Clean(path)
}

// ProjectNamed returns the first project named exactly name.
func (s *Store) ProjectNamed(name string) (Project, error) {
	projects, err := s.Projects()
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.Name == name {
			return p, nil
		}
	}
	return Project{}, ErrNotFound
}
