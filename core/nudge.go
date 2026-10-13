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

// Nudges returns every stale project as of now, in the order they were added.
func (s *Store) Nudges() ([]Nudge, error) {
	// read before Query: the store has one connection, which rows holds until closed
	stale, err := s.StaleThreshold()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT name, path, created_at FROM projects WHERE workspace_id = ? ORDER BY id`, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := s.now()
	var out []Nudge
	for rows.Next() {
		var n Nudge
		var created int64
		if err := rows.Scan(&n.Name, &n.Path, &created); err != nil {
			return nil, err
		}
		n.LastActivity = time.Unix(created, 0)
		if c, ok := lastCommit(n.Path); ok && c.After(n.LastActivity) {
			n.LastActivity = c
		}
		n.Quiet = now.Sub(n.LastActivity)
		if n.Quiet > stale {
			out = append(out, n)
		}
	}
	return out, rows.Err()
}

// lastCommit is the newest commit time on any branch of the repo at dir.
// ok is false when git is missing, dir is not a repo, or it has no commits.
// ponytail: one git process per project per call; cache by HEAD mtime if project counts grow large.
func lastCommit(dir string) (t time.Time, ok bool) {
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--all", "--format=%ct").Output()
	if err != nil {
		return t, false
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return t, false
	}
	return time.Unix(sec, 0), true
}
