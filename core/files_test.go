package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestSkipListAppliesBelowTheProjectAndOnlyToFolders(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	repo, commit := gitRepo(t)
	commit(t0.Add(-day))
	app := filepath.Join(repo, "build", "app") // inside a folder named like a build dir
	touch(t, app, "README", t0.Add(-day))
	git(t, repo, t0.Add(-day), "add", "-A")
	git(t, repo, t0.Add(-day), "commit", "-q", "-m", "app")
	s.AddProject(Project{Name: "app", Path: app})

	touch(t, app, "main.go", t0.Add(3*day))
	touch(t, app, "dist", t0.Add(4*day)) // a file named dist is not a dist folder

	c.t = t0.Add(10 * day)
	st := status(t, s)
	if !st.LastActivity.Equal(t0.Add(4*day)) || st.Note.Lines[len(st.Note.Lines)-1] != "Uncommitted: 2 files" {
		t.Fatalf("got %v %q", st.LastActivity, st.Note.Lines)
	}
}

func TestDeletingAFileIsActivity(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, _ := gitRepo(t)
	touch(t, dir, "old/legacy.go", t0.Add(-day))
	git(t, dir, t0.Add(-day), "add", "-A")
	git(t, dir, t0.Add(-day), "commit", "-q", "-m", "init")
	s.AddProject(Project{Name: "app", Path: dir})

	os.Remove(filepath.Join(dir, "old", "legacy.go"))
	os.Chtimes(filepath.Join(dir, "old"), t0.Add(5*day), t0.Add(5*day)) // a delete touches its folder

	c.t = t0.Add(10 * day)
	if st := status(t, s); !st.LastActivity.Equal(t0.Add(5 * day)) {
		t.Fatalf("got %v %q", st.LastActivity, st.Note.Lines)
	}
}

func TestFutureTimesAndTheStoreItselfAreNotActivity(t *testing.T) {
	home := t.TempDir()
	c := &clock{t0}
	s, err := Open(filepath.Join(home, "config", "proz"), c.now) // the store lives inside the project
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.AddProject(Project{Name: "home", Path: home})
	touch(t, home, "notes.md", t0.Add(2*day))
	touch(t, home, "from-a-tarball.txt", t0.Add(400*day)) // clock skew or an extracted archive

	c.t = t0.Add(10 * day)
	if st := status(t, s); !st.LastActivity.Equal(t0.Add(2 * day)) {
		t.Fatalf("got %v", st.LastActivity)
	}
}

func TestAFolderReplacedByAFileIsMissing(t *testing.T) {
	s := open(t, t.TempDir())
	parent := filepath.Join(t.TempDir(), "parent")
	os.MkdirAll(filepath.Join(parent, "proj"), 0755)
	s.AddProject(Project{Name: "proj", Path: filepath.Join(parent, "proj")})
	os.RemoveAll(parent)
	os.WriteFile(parent, []byte("now a file"), 0644)
	lines := status(t, s).Note.Lines
	if !strings.HasPrefix(lines[len(lines)-1], "Folder missing: ") {
		t.Fatalf("got %q", lines)
	}
}

func TestWalkIsBoundedByEntriesNotJustFiles(t *testing.T) {
	dir := t.TempDir()
	for i := range maxWalk + 50 {
		os.Mkdir(filepath.Join(dir, fmt.Sprintf("d%05d", i)), 0755)
	}
	touch(t, dir, "zzz-last.txt", t0) // sorts after every folder: past the bound
	if w := walkNewest(dir, time.Now(), nil); !w.when.IsZero() {
		t.Fatalf("the walk should stop at maxWalk entries, got %v", w.when)
	}
}
