//go:build windows

package autostart

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Install starts exe's tray at login.
func Install(exe string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("proz", `"`+exe+`" tray`)
}

// Remove stops starting the tray at login. Removing when not installed is fine.
func Remove() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue("proz"); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func Installed() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue("proz")
	return err == nil
}
