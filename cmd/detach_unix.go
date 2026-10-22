//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// ownSession starts cmd in a session of its own, so signals to proz's
// process group (Ctrl+C on the tray, launchd stopping its job) leave it be.
func ownSession(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
