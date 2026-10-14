// Package core holds proz's domain logic and storage. It imports no UI,
// network or frontend package; every frontend calls it directly.
package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultWorkspace is the only workspace until hosted multi-tenancy exists.
const DefaultWorkspace = "default"

type Project struct {
	ID        int64    `json:"-"`
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Programs  []string `json:"programs,omitempty"`
	Arguments []string `json:"arguments"`
}

type Store struct {
	db  *sql.DB
	dir string
	now func() time.Time
}

// migrations run in order; PRAGMA user_version records how many have run.
// Forward-only: never edit a shipped entry, append a new one.
var migrations = []string{
	`CREATE TABLE projects (
		id INTEGER PRIMARY KEY,
		workspace_id TEXT NOT NULL DEFAULT 'default',
		name TEXT NOT NULL,
		path TEXT NOT NULL,
		programs TEXT NOT NULL,
		arguments TEXT NOT NULL,
		created_at INTEGER NOT NULL
	)`,
	`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE recent_programs (
		id INTEGER PRIMARY KEY,
		workspace_id TEXT NOT NULL DEFAULT 'default',
		program TEXT NOT NULL
	)`,
	`CREATE TABLE settings (
		workspace_id TEXT NOT NULL DEFAULT 'default',
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		PRIMARY KEY (workspace_id, key)
	)`,
	`ALTER TABLE projects ADD COLUMN last_launch INTEGER`,
	`ALTER TABLE projects ADD COLUMN stale_seconds INTEGER`, // per-project override; NULL uses the global threshold
	`CREATE TABLE milestones (
		id INTEGER PRIMARY KEY,
		workspace_id TEXT NOT NULL DEFAULT 'default',
		project_id INTEGER NOT NULL,
		text TEXT NOT NULL,
		created_at INTEGER NOT NULL
	)`,
	`CREATE INDEX milestones_by_project ON milestones (project_id, id)`, // latest milestone per project, on every nudges call
	`ALTER TABLE projects ADD COLUMN snoozed_until INTEGER`,
	`ALTER TABLE projects ADD COLUMN dropped_at INTEGER`, // archived while no activity is newer
}

// Open opens (creating if needed) the store in dir. now is the clock every
// time-dependent rule reads, so tests can inject a fake one.
func Open(dir string, now func() time.Time) (*Store, error) {
	if err := os.MkdirAll(dir, 0775); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "proz.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // ponytail: one writer; WAL plus a pool if concurrent agents contend
	s := &Store{db: db, dir: dir, now: now}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.importJSON(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	for ; v < len(migrations); v++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", v+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalJSON migrates the old single "executable" field into Programs.
func (p *Project) UnmarshalJSON(data []byte) error {
	type Alias Project
	aux := struct {
		Executable string `json:"executable"`
		*Alias
	}{Alias: (*Alias)(p)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(p.Programs) == 0 && aux.Executable != "" {
		p.Programs = []string{aux.Executable}
	}
	return nil
}

// importJSON copies the pre-SQLite JSON files in once each. They are left
// untouched as a backup; a corrupt file fails Open so nothing is lost.
func (s *Store) importJSON() error {
	if err := s.importOnce("store.json", func(tx *sql.Tx, data []byte) error {
		var legacy []Project
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		for _, p := range legacy {
			if err := insertProject(tx, p, s.now()); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return s.importOnce("recent.json", func(tx *sql.Tx, data []byte) error {
		var progs []string
		if json.Unmarshal(data, &progs) != nil {
			return nil // recent programs are a convenience; a corrupt file is not worth blocking every command
		}
		for _, prog := range slices.Backward(progs) {
			if err := pushRecent(tx, prog); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) importOnce(file string, load func(*sql.Tx, []byte) error) error {
	key := "imported:" + file
	err := s.db.QueryRow(`SELECT 1 FROM meta WHERE key = ?`, key).Scan(new(int))
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, file))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := load(tx, data); err != nil {
		return fmt.Errorf("%s is corrupt, fix or remove it: %w", file, err)
	}
	// OR IGNORE: a concurrent proz may have imported first; its rows win and ours roll back
	res, err := tx.Exec(`INSERT OR IGNORE INTO meta (key, value) VALUES (?, '1')`, key)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return err
	}
	return tx.Commit()
}

func (s *Store) AddProject(p Project) error {
	return insertProject(s.db, p, s.now())
}

type execer interface {
	Exec(string, ...any) (sql.Result, error)
}

func insertProject(db execer, p Project, now time.Time) error {
	_, err := db.Exec(`INSERT INTO projects (workspace_id, name, path, programs, arguments, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		DefaultWorkspace, p.Name, p.Path, encode(p.Programs), encode(p.Arguments), now.Unix())
	return err
}

var ErrNotFound = errors.New("project not found")

// UpdateProject replaces name, path, programs and arguments of the project with p.ID.
func (s *Store) UpdateProject(p Project) error {
	res, err := s.db.Exec(`UPDATE projects SET name = ?, path = ?, programs = ?, arguments = ? WHERE id = ? AND workspace_id = ?`,
		p.Name, p.Path, encode(p.Programs), encode(p.Arguments), p.ID, DefaultWorkspace)
	return foundOrErr(res, err)
}

func (s *Store) RemoveProject(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM projects WHERE id = ? AND workspace_id = ?`, id, DefaultWorkspace)
	if err := foundOrErr(res, err); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM milestones WHERE project_id = ? AND workspace_id = ?`, id, DefaultWorkspace); err != nil {
		return err
	}
	return tx.Commit()
}

// foundOrErr turns a write that touched no row into ErrNotFound.
func foundOrErr(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// encode stores a string list as JSON text; marshaling []string cannot fail.
func encode(v []string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Projects returns every project in the order they were added.
func (s *Store) Projects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id, name, path, programs, arguments FROM projects WHERE workspace_id = ? ORDER BY id`, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var progs, args string
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &progs, &args); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(progs), &p.Programs); err != nil {
			return nil, fmt.Errorf("project %d programs: %w", p.ID, err)
		}
		if err := json.Unmarshal([]byte(args), &p.Arguments); err != nil {
			return nil, fmt.Errorf("project %d arguments: %w", p.ID, err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const maxRecent = 4

// PushRecentProgram moves program to the front of the recent list, keeping
// at most maxRecent entries.
func (s *Store) PushRecentProgram(program string) error {
	return pushRecent(s.db, program)
}

func pushRecent(db execer, program string) error {
	if _, err := db.Exec(`DELETE FROM recent_programs WHERE workspace_id = ? AND program = ?`, DefaultWorkspace, program); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO recent_programs (workspace_id, program) VALUES (?, ?)`, DefaultWorkspace, program); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM recent_programs WHERE workspace_id = ? AND id NOT IN
		(SELECT id FROM recent_programs WHERE workspace_id = ? ORDER BY id DESC LIMIT ?)`, DefaultWorkspace, DefaultWorkspace, maxRecent)
	return err
}

// RecentPrograms returns recently used programs, newest first.
func (s *Store) RecentPrograms() ([]string, error) {
	rows, err := s.db.Query(`SELECT program FROM recent_programs WHERE workspace_id = ? ORDER BY id DESC`, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Launch records a launch of p (activity, even with no program) and program
// as recently used, and returns the command that opens
// p with it; the caller wires stdio and runs it. No program means nothing to
// launch, so it returns a nil command.
func (s *Store) Launch(p Project, program string) (*exec.Cmd, error) {
	res, err := s.db.Exec(`UPDATE projects SET last_launch = ? WHERE id = ? AND workspace_id = ?`, s.now().Unix(), p.ID, DefaultWorkspace)
	if err := foundOrErr(res, err); err != nil {
		return nil, err
	}
	if program == "" || p.Path == "" { // an idea has no folder to open yet
		return nil, nil
	}
	if err := s.PushRecentProgram(program); err != nil {
		return nil, err
	}
	cmd := exec.Command(program, append(slices.Clone(p.Arguments), p.Path)...)
	cmd.Dir = p.Path
	return cmd, nil
}
