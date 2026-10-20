package core

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
)

// otherCheckouts returns dir's counterpart in every other checkout of its
// git repository (its worktrees, or from inside a worktree the main checkout
// too), as git lists them now; worktrees are never stored.
// ponytail: nil when git is slow or absent, so checks that use it then pass.
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
	for block := range strings.SplitSeq(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n\n") {
		lines := strings.Split(block, "\n")
		root, ok := strings.CutPrefix(lines[0], "worktree ")
		if !ok || slices.Contains(lines, "bare") || resolve(filepath.FromSlash(root)) == resolve(filepath.FromSlash(top)) {
			continue // a bare repository has no files to read
		}
		found = append(found, filepath.Join(filepath.FromSlash(root), filepath.FromSlash(prefix)))
	}
	return found
}
