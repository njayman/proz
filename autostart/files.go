//go:build !windows

package autostart

import (
	"os"
	"runtime"
)

func current() (entry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return entry{}, err
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return entry{}, err
	}
	return entryFor(runtime.GOOS, home, config), nil
}

// Install starts exe's tray at login.
func Install(exe string) error {
	e, err := current()
	if err != nil {
		return err
	}
	return e.install(exe)
}

// Remove stops starting the tray at login. Removing when not installed is fine.
func Remove() error {
	e, err := current()
	if err != nil {
		return err
	}
	return e.remove()
}

func Installed() bool {
	e, err := current()
	return err == nil && e.installed()
}
