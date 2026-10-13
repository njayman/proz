// Package core holds proz's domain logic and storage. It imports no UI,
// network or frontend package; every frontend calls it directly.
package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		if err := json.Unmarshal(data, &progs); err != nil {
			return err
		}
		for i := len(progs) - 1; i >= 0; i-- {
			if err := pushRecent(tx, progs[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) importOnce(file string, load func(*sql.Tx, []byte) error) error {
	key := "imported:" + file
	var done int
	s.db.QueryRow(`SELECT 1 FROM meta WHERE key = ?`, key).Scan(&done)
	if done == 1 {
		return nil
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
	if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES (?, '1')`, key); err != nil {
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
	progs, _ := json.Marshal(p.Programs)
	args, _ := json.Marshal(p.Arguments)
	_, err := db.Exec(`INSERT INTO projects (workspace_id, name, path, programs, arguments, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		DefaultWorkspace, p.Name, p.Path, string(progs), string(args), now.Unix())
	return err
}

var ErrNotFound = errors.New("project not found")

// UpdateProject replaces name, path, programs and arguments of the project with p.ID.
func (s *Store) UpdateProject(p Project) error {
	progs, _ := json.Marshal(p.Programs)
	args, _ := json.Marshal(p.Arguments)
	res, err := s.db.Exec(`UPDATE projects SET name = ?, path = ?, programs = ?, arguments = ? WHERE id = ? AND workspace_id = ?`,
		p.Name, p.Path, string(progs), string(args), p.ID, DefaultWorkspace)
	return affectedOne(res, err)
}

func (s *Store) RemoveProject(id int64) error {
	res, err := s.db.Exec(`DELETE FROM projects WHERE id = ? AND workspace_id = ?`, id, DefaultWorkspace)
	return affectedOne(res, err)
}

func affectedOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
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
		json.Unmarshal([]byte(progs), &p.Programs)
		json.Unmarshal([]byte(args), &p.Arguments)
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
