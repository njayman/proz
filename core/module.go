package core

import (
	"database/sql"
	"errors"
	"fmt"
)

// Module is a part of proz that can be switched on or off.
type Module string

const (
	ModuleNotifications Module = "notifications"
	ModuleTray          Module = "tray"
	ModuleEmail         Module = "email"   // off until SMTP is configured
	ModuleBrain         Module = "brain"   // off until a brain folder is configured
	ModulePrompts       Module = "prompts" // the last Claude Code prompt in notes; off keeps it private
)

var moduleDefaults = map[Module]bool{
	ModuleNotifications: true,
	ModuleTray:          false, // on once the user enables it
	ModuleEmail:         false,
	ModuleBrain:         false,
	ModulePrompts:       true,
}

// Modules lists every module, in a stable order.
var Modules = []Module{ModuleNotifications, ModuleTray, ModuleEmail, ModuleBrain, ModulePrompts}

func (s *Store) ModuleOn(m Module) (bool, error) {
	def, ok := moduleDefaults[m]
	if !ok {
		return false, fmt.Errorf("unknown module %q", m)
	}
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = ?`, DefaultWorkspace, "module."+string(m)).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	return v == "on", err
}

func (s *Store) SetModule(m Module, on bool) error {
	if _, ok := moduleDefaults[m]; !ok {
		return fmt.Errorf("unknown module %q", m)
	}
	if m == ModuleEmail && on {
		if _, ok, err := s.SMTP(); err != nil || !ok {
			return errors.Join(errors.New("configure SMTP first: proz config smtp"), err)
		}
	}
	if m == ModuleBrain && on {
		if dir, err := s.BrainPath(); err != nil || dir == "" {
			return errors.Join(errors.New("set a brain folder first: proz config brain-path <dir>"), err)
		}
	}
	v := "off"
	if on {
		v = "on"
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, ?, ?)`, DefaultWorkspace, "module."+string(m), v); err != nil {
		return err
	}
	if m == ModulePrompts {
		return s.SyncBrain() // brain notes may sync elsewhere: drop or add the prompt now, not at the next check
	}
	return nil
}
