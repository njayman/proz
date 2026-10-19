package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsNewConsoleFlag(t *testing.T) {
	s := open(t, t.TempDir())
	p := project(t, s)
	t.Setenv("PATH", filepath.Join(os.Getenv("SystemRoot"), "System32")) // notepad.exe, a GUI program

	cmd, err := s.Launch(p, "nvim", false)
	if err != nil || cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&createNewConsole == 0 {
		t.Fatalf("terminal program: want a new console, got %+v, %v", cmd.SysProcAttr, err)
	}
	cmd, err = s.Launch(p, "notepad", false)
	if err != nil || (cmd.SysProcAttr != nil && cmd.SysProcAttr.CreationFlags&createNewConsole != 0) {
		t.Fatalf("GUI program: want no new console, got %+v, %v", cmd.SysProcAttr, err)
	}
	cmd, err = s.LaunchTerminal(p, false)
	if err != nil || cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&createNewConsole == 0 {
		t.Fatalf("terminal only: want a new console, got %+v, %v", cmd.SysProcAttr, err)
	}
}
