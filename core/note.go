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
	dir = filepath.Clean(dir)
	projects, err := s.Projects()
	if err != nil {
		return Project{}, err
	}
	var best Project
	bestLen := 0
	for _, p := range projects {
		root := filepath.Clean(p.Path)
		if p.Path == "" || len(root) <= bestLen {
			continue
		}
		if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
			best, bestLen = p, len(root)
		}
	}
	if best.ID == 0 {
		return Project{}, ErrNotFound
	}
	return best, nil
}
