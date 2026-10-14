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

// Status is where a project stands as of now: its newest activity, how long
// it has been quiet, whether that makes it stale, and its left-off note.
type Status struct {
	Project
	LastActivity time.Time
	Quiet        time.Duration
	Stale        bool
	Note         Note
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
func (s *Store) Nudges() ([]Status, error) {
	all, err := s.statuses(0)
	if err != nil {
		return nil, err
	}
	var out []Status
	for _, st := range all {
		if st.Stale {
			out = append(out, st)
		}
	}
	return out, nil
}

// Status returns one project's status.
func (s *Store) Status(id int64) (Status, error) {
	all, err := s.statuses(id)
	if err != nil {
		return Status{}, err
	}
	if len(all) == 0 {
		return Status{}, ErrNotFound
	}
	return all[0], nil
}

// statuses computes the status of project id, or of every project when id is 0.
func (s *Store) statuses(id int64) ([]Status, error) {
	global, err := s.StaleThreshold()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT p.id, p.name, p.path,
			max(p.created_at, coalesce(p.last_launch, 0), coalesce(m.created_at, 0)), p.stale_seconds, m.text
		FROM projects p
		LEFT JOIN milestones m ON m.id = (SELECT max(id) FROM milestones WHERE project_id = p.id AND workspace_id = p.workspace_id)
		WHERE p.workspace_id = ? AND (? = 0 OR p.id = ?) ORDER BY p.id`, DefaultWorkspace, id, id)
	if err != nil {
		return nil, err
	}
	type row struct {
		st        Status
		recorded  int64 // newest of registration, launch and milestone
		threshold sql.NullInt64
		milestone sql.NullString // latest
	}
	var found []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.st.ID, &r.st.Name, &r.st.Path, &r.recorded, &r.threshold, &r.milestone); err != nil {
			rows.Close()
			return nil, err
		}
		found = append(found, r)
	}
	rows.Close() // release the single connection before running git
	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := s.now()
	out := make([]Status, 0, len(found))
	for _, r := range found {
		st := r.st
		st.LastActivity = time.Unix(r.recorded, 0)
		g, hasGit := readGit(st.Path)
		if hasGit && g.when.After(st.LastActivity) {
			st.LastActivity = g.when
		}
		st.Quiet = now.Sub(st.LastActivity)
		threshold := global
		if r.threshold.Valid {
			threshold = time.Duration(r.threshold.Int64) * time.Second
		}
		st.Stale = st.Quiet > threshold

		// Fixed order; later signals insert their line in its slot.
		st.Note.Lines = []string{quietLine(st.Quiet)}
		if r.milestone.Valid {
			st.Note.Lines = append(st.Note.Lines, "Milestone: "+strings.Join(strings.Fields(r.milestone.String), " "))
		}
		if hasGit {
			st.Note.Lines = append(st.Note.Lines, "Last commit: "+g.subject)
			if g.branch != "" {
				st.Note.Lines = append(st.Note.Lines, "Branch: "+g.branch)
			}
		}
		out = append(out, st)
	}
	return out, nil
}

type gitState struct {
	when    time.Time
	subject string
	branch  string // empty when HEAD is detached
}

// readGit reads the newest commit on a local branch (or a detached HEAD)
// that touches dir, and the branch it is on. Stashes and remote-tracking refs
// are not the user's work, and in a bigger repo only commits under dir count.
// ok is false when dir is empty, git is missing, or no such commit exists, so
// the project falls back to its other signals; a nudge is the safe failure.
// ponytail: one or two git processes per project per call; cache by HEAD mtime if project counts grow large.
func readGit(dir string) (g gitState, ok bool) {
	if dir == "" {
		return g, false
	}
	// %S is the ref the commit was reached from, so the branch matches the commit.
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--source", "--branches", "HEAD", "--format=%ct%x00%S%x00%s", "--", ".").Output()
	if err != nil {
		return g, false
	}
	f := strings.SplitN(strings.TrimRight(string(out), "\n"), "\x00", 3)
	if len(f) != 3 {
		return g, false
	}
	n, err := strconv.ParseInt(f[0], 10, 64)
	if err != nil {
		return g, false
	}
	g.when, g.subject = time.Unix(n, 0), f[2]
	if f[1] != "HEAD" {
		g.branch = strings.TrimPrefix(f[1], "refs/heads/") // git prints either form
	} else if out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output(); err == nil {
		g.branch = strings.TrimSpace(string(out)) // reached from HEAD: the checked-out branch, empty if detached
	}
	return g, true
}
