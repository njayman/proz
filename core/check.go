package core

// Check runs one iteration of the resurfacing loop: it hands deliver the
// nudges not yet notified in their current stale period, and marks them
// notified only if deliver succeeds, so a failed delivery is retried and a
// restart never repeats one. The tray loop and 'proz check' both call it.
func (s *Store) Check(deliver func(due []Status) error) error {
	all, err := s.statuses(0)
	if err != nil {
		return err
	}
	var due []Status
	for _, st := range all {
		if st.State == StateStale && st.notified.Before(st.StaleSince) {
			due = append(due, st)
		}
	}
	if len(due) == 0 {
		return nil
	}
	if err := deliver(due); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now().Unix()
	for _, st := range due {
		if _, err := tx.Exec(`UPDATE projects SET notified_at = ? WHERE id = ? AND workspace_id = ?`, now, st.ID, DefaultWorkspace); err != nil {
			return err
		}
	}
	return tx.Commit()
}
