package core

// Check runs one iteration of the resurfacing loop. It claims the nudges not
// yet notified in their current stale period, hands them to deliver, and
// releases the claims deliver did not use. deliver returns how many of due,
// in order, it showed. Claiming first means the tray and a cron job running
// at once never both notify; releasing means a failed delivery is retried;
// the claim is persisted, so a restart never repeats one. The tray loop and
// 'proz check' both call it.
func (s *Store) Check(deliver func(due []Status) (sent int, err error)) error {
	all, err := s.statuses(0)
	if err != nil {
		return err
	}
	now := s.now().Unix()
	var due []Status
	if err := s.inTx(func(tx execer) error {
		for _, st := range all {
			if st.State != StateStale || !st.notified.Before(st.StaleSince) {
				continue
			}
			res, err := tx.Exec(`UPDATE projects SET notified_at = ? WHERE id = ? AND workspace_id = ?
				AND (notified_at IS NULL OR notified_at < ?)`, now, st.ID, DefaultWorkspace, st.StaleSince.Unix())
			if err != nil {
				return err
			}
			if n, err := res.RowsAffected(); err != nil {
				return err
			} else if n == 1 { // 0: another process claimed it first
				due = append(due, st)
			}
		}
		return nil
	}); err != nil || len(due) == 0 {
		return err
	}

	sent, derr := deliver(due)
	if sent < len(due) {
		if err := s.inTx(func(tx execer) error {
			for _, st := range due[max(sent, 0):] {
				var prev any // NULL when never notified
				if !st.notified.IsZero() {
					prev = st.notified.Unix()
				}
				if _, err := tx.Exec(`UPDATE projects SET notified_at = ? WHERE id = ? AND workspace_id = ? AND notified_at = ?`,
					prev, st.ID, DefaultWorkspace, now); err != nil {
					return err
				}
			}
			return nil
		}); err != nil && derr == nil {
			derr = err
		}
	}
	return derr
}

func (s *Store) inTx(fn func(execer) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
