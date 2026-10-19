package core

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// skipDirs never count as activity: dependencies, build output, caches.
// Files git ignores never count either; git status leaves them out.
// ponytail: matched case-sensitively, below the project folder only.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".venv": true, "venv": true,
	"__pycache__": true, "dist": true, "build": true, "target": true, ".next": true,
	".nuxt": true, ".cache": true, ".gradle": true, ".tox": true, ".terraform": true,
}

// maxWalk bounds the walk of a folder that is not a git repository, in
// entries (files and folders).
// ponytail: newest file among the first 10k entries; a huge plain folder may miss a deep edit.
const maxWalk = 10000

type workState struct {
	when        time.Time // newest change among uncommitted (or, without git, all) files
	uncommitted int       // files changed, new or deleted under the project, per git
}

// readWork reads work in progress in dir as of now: in a git repository the
// files git reports as changed, new or deleted; otherwise a bounded walk.
// Times after now (clock skew, extracted archives) and anything under the
// folders in ignore (the proz store, the brain) never count.
// ponytail: --untracked-files=all lists every file of an untracked, unignored dependency folder before it is skipped.
func readWork(ctx context.Context, dir string, now time.Time, ignore []string) (w workState) {
	// one call: the repository root, then dir's path inside it
	out, err := gitCmd(ctx, dir, "rev-parse", "--show-toplevel", "--show-prefix").Output()
	if err != nil { // not a repository, or git too slow: the newest file, capped at maxWalk
		return walkNewest(dir, now, ignore)
	}
	top, prefix, _ := strings.Cut(strings.TrimRight(string(out), "\n"), "\n")
	cmd := gitCmd(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".")
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0") // read only: never contend with the user's git
	status, err := cmd.Output()
	if ctx.Err() != nil {
		return walkNewest(dir, now, ignore)
	}
	if err != nil {
		return w
	}
	entries := strings.Split(string(status), "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		if strings.ContainsAny(e[:2], "RC") {
			i++ // a rename or copy is followed by its original path
		}
		path := filepath.Join(top, filepath.FromSlash(e[3:])) // e[3:] is relative to the repository root
		if skipped(strings.TrimPrefix(e[3:], prefix)) || slices.ContainsFunc(ignore, func(d string) bool { return within(path, d) }) {
			continue
		}
		w.uncommitted++
		info, err := os.Stat(path)
		if err != nil { // deleted: the delete touched its folder
			info, err = os.Stat(filepath.Dir(path))
		}
		if err == nil {
			w.see(info.ModTime(), now)
		}
	}
	return w
}

func (w *workState) see(t, now time.Time) {
	if t.After(w.when) && !t.After(now) {
		w.when = t
	}
}

// skipped reports whether a slash path, relative to the project folder, lies
// in a skipped folder. The last element is the file itself, never skipped.
func skipped(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, p := range parts[:len(parts)-1] {
		if skipDirs[p] {
			return true
		}
	}
	return false
}

func walkNewest(dir string, now time.Time, ignore []string) (w workState) {
	dir = resolve(dir) // ignore holds resolved folders: walk resolved paths too
	seen := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if seen++; seen > maxWalk {
			return filepath.SkipAll
		}
		if err != nil {
			return nil // unreadable: skip, keep walking
		}
		if d.IsDir() {
			if path != dir && (skipDirs[d.Name()] || slices.Contains(ignore, path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if info, err := d.Info(); err == nil {
			w.see(info.ModTime(), now)
		}
		return nil
	})
	return w
}
