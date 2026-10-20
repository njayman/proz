package core

import (
	"context"
	"path/filepath"
	"strings"
)

// otherCheckouts returns dir's counterpart in every other worktree of its
// git repository, as git lists them now; worktrees are never stored.
func otherCheckouts(ctx context.Context, dir string) []string {
	out, err := gitCmd(ctx, dir, "rev-parse", "--show-toplevel", "--show-prefix").Output()
	if err != nil {
		return nil
	}
	top, prefix, _ := strings.Cut(strings.TrimRight(string(out), "\n"), "\n")
	out, err = gitCmd(ctx, dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var found []string
	for line := range strings.Lines(string(out)) {
		root, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "worktree ")
		if !ok || resolve(filepath.FromSlash(root)) == resolve(filepath.FromSlash(top)) {
			continue
		}
		found = append(found, filepath.Join(filepath.FromSlash(root), filepath.FromSlash(prefix)))
	}
	return found
}
