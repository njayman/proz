package core

import (
	"errors"
	"strings"
	"time"
)

// Milestone is a progress checkpoint an agent records on a project, such as
// "shipped login; next: signup". The latest one leads the left-off note.
type Milestone struct {
	Text string
	At   time.Time
}

// AddMilestone records a milestone on project id; it counts as activity and
// refreshes the project's brain note.
func (s *Store) AddMilestone(id int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("milestone text is empty")
	}
	// INSERT ... SELECT only inserts when the project exists in this workspace.
	res, err := s.db.Exec(`INSERT INTO milestones (workspace_id, project_id, text, created_at)
		SELECT workspace_id, id, ?, ? FROM projects WHERE id = ? AND workspace_id = ?`,
		text, s.now().Unix(), id, DefaultWorkspace)
	if err := foundOrErr(res, err); err != nil {
		return err
	}
	s.BrainNote(id) // best effort: the milestone is saved; the next check retries the note
	return nil
}

// Milestones returns project id's milestones, newest first.
func (s *Store) Milestones(id int64) ([]Milestone, error) {
	rows, err := s.db.Query(`SELECT text, created_at FROM milestones WHERE project_id = ? AND workspace_id = ? ORDER BY id DESC`, id, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Milestone
	for rows.Next() {
		var m Milestone
		var at int64
		if err := rows.Scan(&m.Text, &at); err != nil {
			return nil, err
		}
		m.At = time.Unix(at, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}
