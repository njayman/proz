package core

import (
	"database/sql"
	"errors"
	"fmt"
)

// Theme is how the launcher looks: following the system, or always light
// or dark.
type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

// Theme is the launcher's look; ThemeSystem until the user picks one.
func (s *Store) Theme() (Theme, error) {
	var t Theme
	err := s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = 'theme'`, DefaultWorkspace).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return ThemeSystem, nil
	}
	return t, err
}

// SetTheme sets the launcher's look; ThemeSystem follows the system again.
func (s *Store) SetTheme(t Theme) error {
	switch t {
	case ThemeSystem:
		_, err := s.db.Exec(`DELETE FROM settings WHERE workspace_id = ? AND key = 'theme'`, DefaultWorkspace)
		return err
	case ThemeLight, ThemeDark:
		_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, 'theme', ?)`, DefaultWorkspace, string(t))
		return err
	}
	return fmt.Errorf("theme %q: use system, light or dark", t)
}
