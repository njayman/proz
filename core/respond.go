package core

import (
	"errors"
	"time"
)

// Responses to a nudge. Resume is Launch: a launch is activity, which ends
// the stale period and revives an archived project.

// Snooze hides project id's nudge for d. When d passes and the project is
// still stale, it nudges again.
func (s *Store) Snooze(id int64, d time.Duration) error {
	if d < time.Second {
		return errors.New("snooze must be positive")
	}
	return s.setProject(id, `snoozed_until = ?`, s.now().Add(d).Unix())
}

// Drop archives project id: it stops nudging until any new activity, which
// brings it back on its own.
func (s *Store) Drop(id int64) error {
	return s.setProject(id, `dropped_at = ?`, s.now().Unix())
}

// Restore un-archives project id and clears any snooze.
func (s *Store) Restore(id int64) error {
	return s.setProject(id, `dropped_at = NULL, snoozed_until = NULL`)
}

// Archived returns dropped projects with no activity since, oldest first.
func (s *Store) Archived() ([]Status, error) {
	all, err := s.statuses(0)
	if err != nil {
		return nil, err
	}
	var out []Status
	for _, st := range all {
		if st.State == StateArchived {
			out = append(out, st)
		}
	}
	return out, nil
}

func (s *Store) setProject(id int64, set string, args ...any) error {
	res, err := s.db.Exec(`UPDATE projects SET `+set+` WHERE id = ? AND workspace_id = ?`, append(args, id, DefaultWorkspace)...)
	return foundOrErr(res, err)
}
