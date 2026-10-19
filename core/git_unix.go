//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// killTree makes cancelling cmd kill its whole process group. ponytail: in
// its own group, git no longer gets the terminal's Ctrl-C; it still ends at
// the limit.
func killTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
