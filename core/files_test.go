package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// touch writes file under dir with modification time at.
func touch(t *testing.T, dir, file string, at time.Time) {
	t.Helper()
	path := filepath.Join(dir, file)
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(at.String()), 0644)
	os.Chtimes(path, at, at)
}

func TestUncommittedWorkIsActivityAndCounted(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, _ := gitRepo(t)
	touch(t, dir, ".gitignore", t0.Add(-day))
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("secret.log\n"), 0644)
	touch(t, dir, "main.go", t0.Add(-day))
	git(t, dir, t0.Add(-day), "add", "-A")
	git(t, dir, t0.Add(-day), "commit", "-q", "-m", "init")
	git(t, dir, t0, "branch", "-M", "main")
	s.AddProject(Project{Name: "app", Path: dir})

	touch(t, dir, "main.go", t0.Add(5*day))             // modified, tracked
	touch(t, dir, "docs/plan.md", t0.Add(4*day))        // untracked
	touch(t, dir, "secret.log", t0.Add(9*day))          // ignored by git: never counts
	touch(t, dir, "node_modules/x/y.js", t0.Add(9*day)) // dependency dir, not ignored: never counts

	c.t = t0.Add(10 * day)
	st := status(t, s)
	if !st.LastActivity.Equal(t0.Add(5 * day)) {
		t.Fatalf("newest uncommitted file should be the activity, got %v", st.LastActivity)
	}
	want := []string{"Quiet for 5 days", "Last commit: init", "Branch: main", "Uncommitted: 2 files"}
	if !reflect.DeepEqual(st.Note.Lines, want) {
		t.Fatalf("got %q", st.Note.Lines)
	}
}

func TestFolderWithoutGitFallsBackToItsNewestFile(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir := t.TempDir()
	s.AddProject(Project{Name: "notes", Path: dir})
	touch(t, dir, "draft.md", t0.Add(3*day))
	touch(t, dir, "deep/a/b/c.txt", t0.Add(4*day))
	touch(t, dir, "node_modules/pkg/index.js", t0.Add(9*day))
	touch(t, dir, ".venv/lib/x.py", t0.Add(9*day))

	c.t = t0.Add(10 * day)
	st := status(t, s)
	if !st.LastActivity.Equal(t0.Add(4*day)) || len(st.Note.Lines) != 1 {
		t.Fatalf("got %v %q", st.LastActivity, st.Note.Lines)
	}
}

func TestMissingFolderStaysTrackedAndSaysSo(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir := filepath.Join(t.TempDir(), "gone")
	os.MkdirAll(dir, 0755)
	s.AddProject(Project{Name: "gone", Path: dir})
	os.RemoveAll(dir)

	c.t = t0.Add(8 * day)
	ns := nudgesAt(t, s)
	if len(ns) != 1 {
		t.Fatalf("a missing folder still nudges, got %+v", ns)
	}
	lines := ns[0].Note.Lines
	if lines[len(lines)-1] != "Folder missing: "+dir {
		t.Fatalf("got %q", lines)
	}
}

func TestOneUncommittedFileIsSingular(t *testing.T) {
	s := open(t, t.TempDir())
	dir, commit := gitRepo(t)
	commit(t0)
	touch(t, dir, "new.go", t0)
	s.AddProject(Project{Name: "app", Path: dir})
	lines := status(t, s).Note.Lines
	if lines[len(lines)-1] != "Uncommitted: 1 file" {
		t.Fatalf("got %q", lines)
	}
}
