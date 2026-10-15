package core

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The brain is a local markdown folder. proz keeps one note per project in
// it and owns only the section between these markers; everything outside is
// the user's and is never touched.
const (
	brainStart = "<!-- proz:start -->"
	brainEnd   = "<!-- proz:end -->"
)

// ErrNoBrain means the brain is off or has no folder.
var ErrNoBrain = errors.New("no brain folder: proz config brain-path <dir>")

func (s *Store) BrainPath() (string, error) {
	var dir string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = 'brain_path'`, DefaultWorkspace).Scan(&dir)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return dir, err
}

// SetBrainPath points the brain at an existing folder and switches it on.
func (s *Store) SetBrainPath(dir string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("brain folder %s: not an existing folder", dir)
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, 'brain_path', ?)`, DefaultWorkspace, dir); err != nil {
		return err
	}
	return s.SetModule(ModuleBrain, true)
}

func (s *Store) brainDir() (string, error) {
	on, err := s.ModuleOn(ModuleBrain)
	if err != nil {
		return "", err
	}
	dir, err := s.BrainPath()
	if err != nil {
		return "", err
	}
	if !on || dir == "" {
		return "", ErrNoBrain
	}
	return dir, nil
}

// SyncBrain rewrites every project's section in the brain; with no brain it
// does nothing.
func (s *Store) SyncBrain() error {
	dir, err := s.brainDir()
	if errors.Is(err, ErrNoBrain) {
		return nil
	}
	if err != nil {
		return err
	}
	all, err := s.statuses(0)
	if err != nil {
		return err
	}
	var errs []error
	for _, st := range all {
		if _, err := s.syncNote(dir, st); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", st.Name, err))
		}
	}
	return errors.Join(errs...)
}

// BrainNote brings project id's note up to date and returns its path and
// whole content, the user's text included.
func (s *Store) BrainNote(id int64) (path, content string, err error) {
	dir, err := s.brainDir()
	if err != nil {
		return "", "", err
	}
	st, err := s.Status(id)
	if err != nil {
		return "", "", err
	}
	if path, err = s.syncNote(dir, st); err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(path)
	return path, string(data), err
}

// syncNote rewrites st's section in its note, writing only on a change.
func (s *Store) syncNote(dir string, st Status) (string, error) {
	name, err := s.noteName(st.Project)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real // a linked note is written where it lives
	}
	section, err := s.brainSection(st)
	if err != nil {
		return "", err
	}
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	updated := spliceSection(old, section)
	if bytes.Equal(updated, old) {
		return path, nil
	}
	return path, writeKeepingMode(path, updated)
}

// noteName is the project's note file, chosen once and kept across renames;
// a name already used by another project gets the project id appended.
func (s *Store) noteName(p Project) (string, error) {
	var name sql.NullString
	if err := s.db.QueryRow(`SELECT brain_note FROM projects WHERE id = ? AND workspace_id = ?`, p.ID, DefaultWorkspace).Scan(&name); err != nil {
		return "", err
	}
	if name.Valid {
		return name.String, nil
	}
	base := slug(p.Name)
	if base == "" {
		base = "project"
	}
	n := base + ".md"
	var taken int
	if err := s.db.QueryRow(`SELECT count(*) FROM projects WHERE workspace_id = ? AND lower(brain_note) = ?`, DefaultWorkspace, n).Scan(&taken); err != nil {
		return "", err
	}
	if taken > 0 {
		n = base + "-" + strconv.FormatInt(p.ID, 10) + ".md"
	}
	_, err := s.db.Exec(`UPDATE projects SET brain_note = ? WHERE id = ? AND workspace_id = ?`, n, p.ID, DefaultWorkspace)
	return n, err
}

// markerSafe keeps a commit message or milestone quoting a marker from
// ending the section early.
var markerSafe = strings.NewReplacer(brainStart, "[proz:start]", brainEnd, "[proz:end]")

func (s *Store) brainSection(st Status) (string, error) {
	ms, err := s.Milestones(st.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("## proz\n\n")
	if st.Path != "" {
		b.WriteString("Folder: " + st.Path + "\n\n")
	}
	for _, l := range st.Note.Lines {
		b.WriteString("- " + l + "\n")
	}
	if len(ms) > 0 {
		b.WriteString("\n### Milestones\n\n")
		for _, m := range ms {
			b.WriteString("- " + m.At.UTC().Format("2006-01-02") + ": " + strings.Join(strings.Fields(m.Text), " ") + "\n")
		}
	}
	return brainStart + "\n" + markerSafe.Replace(b.String()) + brainEnd, nil
}

// spliceSection replaces the last start marker and the first end marker
// after it with section, or appends section when there is no such pair.
// Only text between a start and its end is ever replaced.
func spliceSection(old []byte, section string) []byte {
	s := string(old)
	if i := strings.LastIndex(s, brainStart); i >= 0 {
		if j := strings.Index(s[i:], brainEnd); j >= 0 {
			return []byte(s[:i] + section + s[i+j+len(brainEnd):])
		}
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	if s != "" {
		s += "\n"
	}
	return []byte(s + section + "\n")
}

// writeKeepingMode replaces path atomically, keeping its permissions (0644
// for a new note), so an editor never sees a half-written note.
func writeKeepingMode(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".proz-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), mode); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
