package core

import (
	"os/exec"
	"syscall"
)

const createNewConsole = 0x00000010 // CREATE_NEW_CONSOLE

// setNewConsole starts cmd with a console of its own, which Windows hands to
// the user's default terminal.
func setNewConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
}
