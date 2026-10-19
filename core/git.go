package core

import (
	"context"
	"os/exec"
	"time"
)

// gitLimit is how long one project's git calls may take in all. A project
// that hits it gets its status from its other signals.
var gitLimit = 3 * time.Second

// gitCmd is a git command in dir that is killed, with any helpers it started
// (git-lfs), when ctx ends.
func gitCmd(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	killTree(cmd)
	cmd.WaitDelay = time.Second // a helper still holding the output pipe cannot stall Wait
	return cmd
}
