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
	ModuleEmail         Module = "email" // off until SMTP is configured
	ModuleBrain         Module = "brain" // off until a brain folder is configured
)

var moduleDefaults = map[Module]bool{
	ModuleNotifications: true,
	ModuleTray:          false, // on once the user enables it
	ModuleEmail:         false,
	ModuleBrain:         false,
}

// Modules lists every module, in a stable order.
var Modules = []Module{ModuleNotifications, ModuleTray, ModuleEmail, ModuleBrain}

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
	v := "off"
	if on {
		v = "on"
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, ?, ?)`, DefaultWorkspace, "module."+string(m), v)
	return err
}
