package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// SMTP is how the email digest is sent. It holds credentials, so it lives in
// an owner-only file in the config directory, never in the database (ADR 0001).
type SMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	From     string `json:"from"`
	To       string `json:"to"`
}

const smtpFile = "smtp.json"

const digestRetry = time.Hour

// SMTP reads the SMTP settings; ok is false when none are configured.
func (s *Store) SMTP() (c SMTP, ok bool, err error) {
	path := filepath.Join(s.dir, smtpFile)
	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		return c, false, fmt.Errorf("%s holds a password but others can read it; run: chmod 600 %s", path, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, false, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, false, fmt.Errorf("%s: %w", smtpFile, err)
	}
	return c, true, nil
}

// SetSMTP validates and saves the SMTP settings, readable only by the owner.
func (s *Store) SetSMTP(c SMTP) error {
	if c.Host == "" {
		return errors.New("smtp host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("smtp port must be 1 to 65535, got %d", c.Port)
	}
	for _, a := range []struct{ name, v string }{{"from", c.From}, {"to", c.To}} {
		if _, err := mail.ParseAddress(a.v); err != nil {
			return fmt.Errorf("%s address %q: %v", a.name, a.v, err)
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// CreateTemp makes the file 0600 before any secret is written to it.
	f, err := os.CreateTemp(s.dir, smtpFile+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(s.dir, smtpFile))
}

// DigestDay is the weekday the email digest goes out, Monday by default.
func (s *Store) DigestDay() (time.Weekday, error) {
	var d int
	err := s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = 'digest_day'`, DefaultWorkspace).Scan(&d)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Monday, nil
	}
	return time.Weekday(d), err
}

func (s *Store) SetDigestDay(d time.Weekday) error {
	if d < time.Sunday || d > time.Saturday {
		return fmt.Errorf("not a weekday: %d", d)
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, 'digest_day', ?)`, DefaultWorkspace, int(d))
	return err
}

// Digest sends at most one digest a week: from the digest day on (so a
// machine that was off that day catches up), the first call hands every
// stale project to deliver. A week with nothing stale is done without a
// send. The send is claimed in the database before delivering, so the tray
// and a cron job never both send and a restart never resends; a failed
// delivery releases the claim to be retried an hour later. The send time
// is a workspace setting, not local bookkeeping, so a synced workspace sends
// one digest, not one per machine.
func (s *Store) Digest(deliver func(stale []Status) error) error {
	day, err := s.DigestDay()
	if err != nil {
		return err
	}
	now := s.now()
	back := (int(now.Weekday()) - int(day) + 7) % 7
	week := time.Date(now.Year(), now.Month(), now.Day()-back, 0, 0, 0, 0, now.Location()).Unix()

	var retry int64 // after a failed send, wait before trying again
	err = s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = 'digest_retry_at'`, DefaultWorkspace).Scan(&retry)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if now.Unix() < retry {
		return nil
	}

	var prev sql.NullString
	err = s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = 'digest_sent_at'`, DefaultWorkspace).Scan(&prev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// A send this week, or within six days (the digest day just moved), is
	// this week's digest. The very first one waits for the digest day itself.
	before := min(week, now.Add(-6*24*time.Hour).Unix())
	if sent, _ := strconv.ParseInt(prev.String, 10, 64); prev.Valid && sent >= before || !prev.Valid && back != 0 {
		return nil
	}
	stale, err := s.Nudges()
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`INSERT INTO settings (workspace_id, key, value) VALUES (?, 'digest_sent_at', ?)
		ON CONFLICT (workspace_id, key) DO UPDATE SET value = excluded.value WHERE CAST(value AS INTEGER) < ?`,
		DefaultWorkspace, now.Unix(), before)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 { // 0: another process claimed it first
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	derr := deliver(stale)
	if derr == nil {
		return nil
	}
	// Release the claim, but wait an hour: retrying a wrong password every
	// tray tick could get the account locked.
	err = s.inTx(func(tx execer) error {
		var err error
		if prev.Valid {
			_, err = tx.Exec(`UPDATE settings SET value = ? WHERE workspace_id = ? AND key = 'digest_sent_at'`, prev.String, DefaultWorkspace)
		} else {
			_, err = tx.Exec(`DELETE FROM settings WHERE workspace_id = ? AND key = 'digest_sent_at'`, DefaultWorkspace)
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, 'digest_retry_at', ?)`,
			DefaultWorkspace, now.Add(digestRetry).Unix())
		return err
	})
	return errors.Join(derr, err)
}
