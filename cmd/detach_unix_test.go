//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
	"testing"
)

// A program the tray starts must outlive the tray: Ctrl+C on the tray, or
// launchd stopping its job, signals the tray's process group, not this one.
func TestStartDetachedLeavesOurProcessGroup(t *testing.T) {
	cmd := exec.Command("sleep", "5")
	if err := startDetached(cmd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill() })
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil || pgid == syscall.Getpgrp() {
		t.Fatalf("child process group %d, ours %d, %v", pgid, syscall.Getpgrp(), err)
	}
}
