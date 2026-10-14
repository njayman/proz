package core

import (
	"database/sql"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultStale = 7 * 24 * time.Hour

// Nudge is a project that has been quiet longer than its stale threshold.
type Nudge struct {
	Name         string
	Path         string
	LastActivity time.Time
	Quiet        time.Duration
}

// StaleThreshold is how long a project may be quiet before it nudges.
func (s *Store) StaleThreshold() (time.Duration, error) {
	var sec int64
	err := s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = 'stale_seconds'`, DefaultWorkspace).Scan(&sec)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultStale, nil
	}
	return time.Duration(sec) * time.Second, err
}

func (s *Store) SetStaleThreshold(d time.Duration) error {
	if d < time.Second {
		return errors.New("stale threshold must be positive")
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, 'stale_seconds', ?)`,
		DefaultWorkspace, int64(d/time.Second))
	return err
}

// SetProjectStaleThreshold overrides the stale threshold for one project;
// d == 0 removes the override so the global threshold applies again.
func (s *Store) SetProjectStaleThreshold(id int64, d time.Duration) error {
	var sec any // NULL clears
	if d != 0 {
		if d < time.Second {
			return errors.New("stale threshold must be positive")
		}
		sec = int64(d / time.Second)
	}
	res, err := s.db.Exec(`UPDATE projects SET stale_seconds = ? WHERE id = ? AND workspace_id = ?`, sec, id, DefaultWorkspace)
	return foundOrErr(res, err)
}

// ProjectStaleThreshold is the threshold that applies to one project: its
// override if set, otherwise the global threshold.
func (s *Store) ProjectStaleThreshold(id int64) (time.Duration, error) {
	var sec sql.NullInt64
	err := s.db.QueryRow(`SELECT stale_seconds FROM projects WHERE id = ? AND workspace_id = ?`, id, DefaultWorkspace).Scan(&sec)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if !sec.Valid {
		return s.StaleThreshold()
	}
	return time.Duration(sec.Int64) * time.Second, nil
}

// Nudges returns every stale project as of now, in the order they were added.
func (s *Store) Nudges() ([]Nudge, error) {
	// read before Query: the store has one connection, which rows holds until closed
	stale, err := s.StaleThreshold()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT name, path, max(created_at, coalesce(last_launch, 0)), stale_seconds FROM projects WHERE workspace_id = ? ORDER BY id`, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := s.now()
	var out []Nudge
	for rows.Next() {
		var n Nudge
		var recorded int64 // newest of registration and launch
		var override sql.NullInt64
		if err := rows.Scan(&n.Name, &n.Path, &recorded, &override); err != nil {
			return nil, err
		}
		n.LastActivity = time.Unix(recorded, 0)
		if c, ok := lastCommit(n.Path); ok && c.After(n.LastActivity) {
			n.LastActivity = c
		}
		n.Quiet = now.Sub(n.LastActivity)
		threshold := stale
		if override.Valid {
			threshold = time.Duration(override.Int64) * time.Second
		}
		if n.Quiet > threshold {
			out = append(out, n)
		}
	}
	return out, rows.Err()
}

// lastCommit is the newest commit on a local branch (or a detached HEAD)
// that touches dir. Stashes and remote-tracking refs are not the user's
// work, and in a bigger repo only commits under dir count. ok is false when
// dir is empty, git is missing, or no such commit exists, so the project
// falls back to its other signals; a nudge is the safe failure.
// ponytail: one git process per project per call; cache by HEAD mtime if project counts grow large.
func lastCommit(dir string) (t time.Time, ok bool) {
	if dir == "" {
		return t, false
	}
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--branches", "HEAD", "--format=%ct", "--", ".").Output()
	if err != nil {
		return t, false
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return t, false
	}
	return time.Unix(sec, 0), true
}
