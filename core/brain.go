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
	"unicode/utf8"
)

// The brain is a local markdown folder. proz keeps one note per project in
// it and owns only the section between these markers; everything outside is
// the user's and is never touched.
const (
	brainStart = "<!-- proz:start -->"
	brainEnd   = "<!-- proz:end -->"
)

// ErrNoBrain means the brain is off or has no folder.
var ErrNoBrain = errors.New("no brain")

// beforeRename runs between preparing a note and replacing it; tests use it
// to save the note "from an editor" in that window.
var beforeRename = func() {}

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
	if dir == "" {
		return "", fmt.Errorf("%w folder set: proz config brain-path <dir>", ErrNoBrain)
	}
	if !on {
		return "", fmt.Errorf("%w: it is switched off: proz config brain on", ErrNoBrain)
	}
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("brain folder %s is missing: %w", dir, err)
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

// syncNote rewrites st's section in its note, writing only on a change. A
// note saved by someone else while proz prepares it is left for next time.
func (s *Store) syncNote(dir string, st Status) (string, error) {
	name, err := s.noteName(st.Project)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		// a linked note is written where it lives; a broken link is never replaced
		if path, err = filepath.EvalSymlinks(path); err != nil {
			return "", fmt.Errorf("note %s links to a missing file: %w", name, err)
		}
	}
	section, err := s.brainSection(st)
	if err != nil {
		return "", err
	}
	before, _ := os.Stat(path)
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	updated := spliceSection(old, section)
	if bytes.Equal(updated, old) {
		return path, nil
	}
	return path, writeIfUnchanged(path, updated, before)
}

// noteName is the project's note file, chosen once and kept across renames;
// a name another project holds (a unique index decides, even across
// processes) gets the project id appended.
func (s *Store) noteName(p Project) (string, error) {
	var name sql.NullString
	get := func() error {
		return s.db.QueryRow(`SELECT brain_note FROM projects WHERE id = ? AND workspace_id = ?`, p.ID, DefaultWorkspace).Scan(&name)
	}
	if err := get(); err != nil || name.Valid {
		return name.String, err
	}
	base := slug(p.Name)
	for len(base) > maxNoteName { // keep under file name limits, on a rune boundary
		_, size := utf8.DecodeLastRuneInString(base)
		base = base[:len(base)-size]
	}
	base = strings.TrimSuffix(base, "-")
	if base == "" {
		base = "project"
	}
	for _, n := range []string{base + ".md", base + "-" + strconv.FormatInt(p.ID, 10) + ".md"} {
		_, err := s.db.Exec(`UPDATE projects SET brain_note = ? WHERE id = ? AND workspace_id = ? AND brain_note IS NULL`, n, p.ID, DefaultWorkspace)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "UNIQUE") {
			return "", err
		}
	}
	if err := get(); err != nil || !name.Valid {
		return "", errors.Join(errors.New("no free note name"), err)
	}
	return name.String, nil
}

const maxNoteName = 100

// markerSafe keeps a commit message or milestone quoting a marker from
// ending the section early.
var markerSafe = strings.NewReplacer(brainStart, "[proz:start]", brainEnd, "[proz:end]")

func (s *Store) brainSection(st Status) (string, error) {
	ms, err := s.Milestones(st.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if st.Path != "" {
		b.WriteString("Folder: " + st.Path + "\n\n")
	}
	// The date, not "Quiet for N days", so a note changes only with activity.
	b.WriteString("- Last activity: " + st.LastActivity.UTC().Format("2006-01-02") + "\n")
	for _, l := range st.Note.Lines[1:] { // the first is the quiet line
		b.WriteString("- " + l + "\n")
	}
	if len(ms) > 0 {
		b.WriteString("\n### Milestones\n\n")
		for _, m := range ms {
			b.WriteString("- " + m.At.UTC().Format("2006-01-02") + ": " + strings.Join(strings.Fields(m.Text), " ") + "\n")
		}
	}
	return brainOpening + markerSafe.Replace(b.String()) + brainEnd, nil
}

// brainOpening starts every section proz writes; a bare start marker the
// user pasted is not one.
const brainOpening = brainStart + "\n## proz\n\n"

// spliceSection replaces the last section proz wrote, from its opening to
// the first end marker after it, or appends section when there is none.
// Only text inside such a section is ever replaced.
func spliceSection(old []byte, section string) []byte {
	s := string(old)
	if i := strings.LastIndex(s, brainOpening); i >= 0 {
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

// writeIfUnchanged replaces path atomically, keeping its permissions (0644
// for a new note), so an editor never sees a half-written note. If the note
// changed since before was taken, it is left alone for the next sync.
func writeIfUnchanged(path string, data []byte, before os.FileInfo) error {
	mode := os.FileMode(0o644)
	if before != nil {
		mode = before.Mode().Perm()
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
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), mode); err != nil {
		return err
	}
	beforeRename()
	// ponytail: a save in the instant after this check is still lost; a lock the editor honours does not exist.
	now, err := os.Stat(path)
	if before == nil && err == nil || before != nil && (err != nil || !now.ModTime().Equal(before.ModTime()) || now.Size() != before.Size()) {
		return nil
	}
	return os.Rename(f.Name(), path)
}
