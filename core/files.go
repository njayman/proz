package core

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// skipDirs never count as activity: dependencies, build output, caches.
// Files git ignores never count either; git status leaves them out.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".venv": true, "venv": true,
	"__pycache__": true, "dist": true, "build": true, "target": true, ".next": true,
	".nuxt": true, ".cache": true, ".gradle": true, ".tox": true, ".terraform": true,
}

// maxWalk bounds the walk of a folder that is not a git repository.
// ponytail: newest file among the first 10k; a huge plain folder may miss a deep edit.
const maxWalk = 10000

type workState struct {
	when        time.Time // newest modification among uncommitted (or, without git, all) files
	uncommitted int       // files changed or new under the project, per git
}

// readWork reads work in progress in dir: in a git repository, the files
// git reports as changed or untracked; otherwise a bounded walk.
func readWork(dir string) (w workState) {
	top, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return walkNewest(dir)
	}
	root := strings.TrimSpace(string(top))
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".").Output()
	if err != nil {
		return w
	}
	entries := strings.Split(string(out), "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		if e[0] == 'R' || e[0] == 'C' {
			i++ // a rename or copy is followed by its original path
		}
		path := e[3:] // relative to the repository root, always with '/'
		if skipped(path) {
			continue
		}
		w.uncommitted++
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil && info.ModTime().After(w.when) {
			w.when = info.ModTime()
		}
	}
	return w
}

func skipped(slashPath string) bool {
	for part := range strings.SplitSeq(slashPath, "/") {
		if skipDirs[part] {
			return true
		}
	}
	return false
}

func walkNewest(dir string) (w workState) {
	seen := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: skip, keep walking
		}
		if d.IsDir() {
			if path != dir && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if seen++; seen > maxWalk {
			return filepath.SkipAll
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(w.when) {
			w.when = info.ModTime()
		}
		return nil
	})
	return w
}
