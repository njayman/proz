package core

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultStale = 7 * 24 * time.Hour

// Status is where a project stands as of now: its newest activity, how long
// it has been quiet, whether that makes it stale, and its left-off note.
type Status struct {
	Project
	LastActivity time.Time
	Quiet        time.Duration
	Stale        bool // quiet longer than the threshold, whatever the State
	State        State
	SnoozedUntil time.Time // zero unless snoozed
	StaleSince   time.Time // when the current stale period began; zero unless Stale
	Note         Note

	notified time.Time // last notification, zero if never
}

// State is what proz does about a project right now. Only StateStale nudges.
type State string

const (
	StateActive   State = "active"
	StateStale    State = "stale"
	StateSnoozed  State = "snoozed"  // stale, but the user deferred it
	StateArchived State = "archived" // dropped, and nothing has happened since
)

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
		if st.State == StateStale {
			out = append(out, st)
		}
	}
	return out, nil
}

// Statuses returns every project's status, in the order they were added.
func (s *Store) Statuses() ([]Status, error) { return s.statuses(0) }

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
	folders, err := s.folders()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT p.id, p.name, `+mainPath+`,
			max(p.created_at, coalesce(p.last_launch, 0), coalesce(m.created_at, 0)), p.stale_seconds, m.text,
			p.snoozed_until, p.snoozed_at, p.dropped_at, p.notified_at
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
		snoozed   sql.NullInt64  // until
		snoozedAt sql.NullInt64
		dropped   sql.NullInt64
		notified  sql.NullInt64
	}
	var found []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.st.ID, &r.st.Name, &r.st.Path, &r.recorded, &r.threshold, &r.milestone, &r.snoozed, &r.snoozedAt, &r.dropped, &r.notified); err != nil {
			rows.Close()
			return nil, err
		}
		r.st.Folders = folders[r.st.ID]
		found = append(found, r)
	}
	rows.Close() // release the single connection before running git
	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := s.now()
	ignore := []string{resolve(s.dir)} // proz's own writes are not the user's activity
	if brain, err := s.BrainPath(); err != nil {
		return nil, err
	} else if brain != "" {
		ignore = append(ignore, resolve(brain))
	}
	prompts, err := s.ModuleOn(ModulePrompts)
	if err != nil {
		return nil, err
	}
	// Read each project's signals in parallel, each under one git deadline
	// for all its folders, so a slow repository costs at most the limit and
	// slow ones do not add up.
	type signals struct {
		g       gitState // from the most recently active folder with git
		hasGit  bool
		missing []string // folders that are gone; skipped, never removed
		work    workState
		cl      claudeState
		hasCl   bool
		slow    bool // git hit the limit
	}
	sig := make([]signals, len(found))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // ponytail: 8 projects at a time, so git processes do not swamp the machine
	for i, r := range found {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), gitLimit)
			defer cancel()
			x := &sig[i]
			var gitActive time.Time // when the folder x.g came from was last active
			// ponytail: a folder nested in another folder of the same project counts its uncommitted files twice
			for _, f := range r.st.Folders {
				if f.Missing {
					x.missing = append(x.missing, f.Path)
					continue
				}
				g, hasGit := readGit(ctx, f.Path)
				work := readWork(ctx, f.Path, now, ignore)
				cl, hasCl := readClaude(f.Path)
				active := work.when
				if hasGit && g.when.After(active) {
					active = g.when
				}
				if hasCl && cl.when.After(active) {
					active = cl.when
				}
				if hasGit && (!x.hasGit || active.After(gitActive)) {
					x.g, x.hasGit, gitActive = g, true, active
				}
				if work.when.After(x.work.when) {
					x.work.when = work.when
				}
				x.work.uncommitted += work.uncommitted
				if hasCl && (!x.hasCl || cl.when.After(x.cl.when)) {
					x.cl, x.hasCl = cl, true
				}
			}
			x.slow = ctx.Err() != nil
		}()
	}
	wg.Wait()

	out := make([]Status, 0, len(found))
	for i, r := range found {
		st := r.st
		st.LastActivity = time.Unix(r.recorded, 0)
		x := sig[i]
		g, hasGit, work := x.g, x.hasGit, x.work
		if hasGit && g.when.After(st.LastActivity) {
			st.LastActivity = g.when
		}
		if work.when.After(st.LastActivity) {
			st.LastActivity = work.when
		}
		cl, hasClaude := x.cl, x.hasCl
		if hasClaude && cl.when.After(st.LastActivity) {
			st.LastActivity = cl.when
		}
		st.Quiet = now.Sub(st.LastActivity)
		threshold := global
		if r.threshold.Valid {
			threshold = time.Duration(r.threshold.Int64) * time.Second
		}
		st.Stale = st.Quiet > threshold
		if r.snoozed.Valid && !st.LastActivity.After(time.Unix(r.snoozedAt.Int64, 0)) {
			st.SnoozedUntil = time.Unix(r.snoozed.Int64, 0)
		}
		if st.Stale {
			// a snooze ending starts a new stale period, so a new notification
			st.StaleSince = st.LastActivity.Add(threshold)
			if st.SnoozedUntil.After(st.StaleSince) {
				st.StaleSince = st.SnoozedUntil
			}
		}
		if r.notified.Valid {
			st.notified = time.Unix(r.notified.Int64, 0)
		}
		switch {
		case r.dropped.Valid && !st.LastActivity.After(time.Unix(r.dropped.Int64, 0)):
			st.State = StateArchived
		case !st.Stale:
			st.State = StateActive
		case now.Before(st.SnoozedUntil):
			st.State = StateSnoozed
		default:
			st.State = StateStale
		}

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
		if work.uncommitted > 0 {
			st.Note.Lines = append(st.Note.Lines, plural("Uncommitted: %d file", work.uncommitted))
		}
		if x.slow {
			st.Note.Lines = append(st.Note.Lines, "Git too slow to read changes")
		}
		if hasClaude && prompts && cl.prompt != "" {
			st.Note.Lines = append(st.Note.Lines, "Last prompt: "+cl.prompt)
		}
		for _, f := range x.missing {
			st.Note.Lines = append(st.Note.Lines, "Folder missing: "+f)
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
// ponytail: two or three git processes per project per call; cache by HEAD mtime if project counts grow large.
func readGit(ctx context.Context, dir string) (g gitState, ok bool) {
	if dir == "" {
		return g, false
	}
	// %S is a ref the commit was reached from, so the branch matches the commit.
	out, err := gitCmd(ctx, dir, "log", "-1", "--source", "--branches", "HEAD", "--format=%ct%x00%H%x00%S%x00%s", "--", ".").Output()
	if err != nil {
		return g, false
	}
	f := strings.SplitN(strings.TrimRight(string(out), "\n"), "\x00", 4)
	if len(f) != 4 {
		return g, false
	}
	n, err := strconv.ParseInt(f[0], 10, 64)
	if err != nil {
		return g, false
	}
	g.when, g.subject = time.Unix(n, 0), f[3]
	g.branch = strings.TrimPrefix(f[2], "refs/heads/") // git prints either form
	// A commit on several branches names the first in git's order; the
	// checked-out one wins when it holds the commit. Empty if detached.
	if out, err := gitCmd(ctx, dir, "branch", "--show-current").Output(); err == nil {
		if cur := strings.TrimSpace(string(out)); g.branch == "HEAD" || cur != "" && cur != g.branch &&
			gitCmd(ctx, dir, "merge-base", "--is-ancestor", f[1], "HEAD").Run() == nil {
			g.branch = cur
		}
	}
	if g.branch == "HEAD" {
		g.branch = ""
	}
	return g, true
}
