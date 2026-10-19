package core

import "os/exec"

// killTree is the default kill on Windows. ponytail: helpers git started
// may outlive it; a job object kills the tree if that shows up.
func killTree(*exec.Cmd) {}
