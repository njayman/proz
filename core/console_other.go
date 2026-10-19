//go:build !windows

package core

import "os/exec"

// setNewConsole is Windows only.
func setNewConsole(*exec.Cmd) {}
