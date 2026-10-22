package core

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
)

// AddIdea captures an idea: a project with no folder yet, named by its text.
// It resurfaces like any project, from registration, launches and milestones.
// It returns the idea's name: text on one line, spaces collapsed.
func (s *Store) AddIdea(text string) (string, error) {
	name, err := ProjectName(text)
	if err != nil {
		return "", err
	}
	return name, s.AddProject(Project{Name: name})
}

// ErrNoName refuses a project or idea with a blank name.
var ErrNoName = errors.New("a project needs a name")

// ProjectName is text as a project's name: one line, spaces collapsed.
func ProjectName(text string) (string, error) {
	name := strings.Join(strings.Fields(text), " ")
	if name == "" {
		return "", ErrNoName
	}
	return name, nil
}

// IdeaFor returns the idea whose name matches the folder dir's name, so
// registering that folder can attach it to the idea instead of creating a
// second project. "Habit Tracker" matches habit-tracker and habit_tracker.
func (s *Store) IdeaFor(dir string) (Project, error) {
	want := slug(filepath.Base(dir))
	projects, err := s.Projects()
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.Path == "" && want != "" && slug(p.Name) == want {
			return p, nil
		}
	}
	return Project{}, ErrNotFound
}

// slug lowercases s and joins its runs of letters and digits with "-".
func slug(s string) string {
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.Join(words, "-")
}
