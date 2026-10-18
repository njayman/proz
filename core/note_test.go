package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func status(t *testing.T, s *Store) Status {
	t.Helper()
	st, err := s.Status(onlyProject(t, s).ID)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestNoteForGitProjectInFixedOrder(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, commit := gitRepo(t)
	commit(t0.Add(-day))
	git(t, dir, t0, "checkout", "-q", "-b", "feature/login")
	gitCommit(t, dir, "login.go", t0.Add(day))
	s.AddProject(Project{Name: "app", Path: dir})

	c.t = t0.Add(9*day + 3*hour)
	st := status(t, s)
	want := []string{"Quiet for 8 days", "Last commit: work", "Branch: feature/login"}
	if !reflect.DeepEqual(st.Note.Lines, want) {
		t.Fatalf("got %q, want %q", st.Note.Lines, want)
	}
	if !st.Stale {
		t.Fatal("8 quiet days should be stale")
	}
	if got := st.Note.Short(); got != "Quiet for 8 days; Last commit: work; Branch: feature/login" {
		t.Fatalf("short: %q", got)
	}
	if got := st.Note.Full(); got != strings.Join(want, "\n") {
		t.Fatalf("full: %q", got)
	}
}

func TestNoteWithoutGitIsQuietDurationOnly(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "notes", Path: t.TempDir()})
	for _, tc := range []struct {
		after time.Duration
		want  string
	}{
		{20 * time.Minute, "Quiet for less than an hour"},
		{5 * hour, "Quiet for 5 hours"},
		{day + hour, "Quiet for 1 day"},
	} {
		c.t = t0.Add(tc.after)
		if st := status(t, s); !reflect.DeepEqual(st.Note.Lines, []string{tc.want}) || st.Stale {
			t.Fatalf("after %v: got %q stale=%v", tc.after, st.Note.Lines, st.Stale)
		}
	}
}

func TestShortNoteIsOneTruncatedLine(t *testing.T) {
	n := Note{Lines: []string{"Quiet for 8 days", "Last commit: " + strings.Repeat("x", 300)}}
	short := n.Short()
	if strings.Contains(short, "\n") || len([]rune(short)) != shortNoteLen || !strings.HasSuffix(short, "...") {
		t.Fatalf("got %d runes: %q", len([]rune(short)), short)
	}
	if !strings.Contains(n.Full(), strings.Repeat("x", 300)) {
		t.Fatal("full note must not truncate")
	}
}

func TestProjectAtResolvesTheNearestEnclosingProject(t *testing.T) {
	s := open(t, t.TempDir())
	root := t.TempDir()
	web, api := filepath.Join(root, "web"), filepath.Join(root, "web", "api")
	s.AddProject(Project{Name: "web", Path: web})
	s.AddProject(Project{Name: "api", Path: api})

	for dir, want := range map[string]string{
		web:                            "web",
		filepath.Join(web, "docs"):     "web",
		filepath.Join(api, "src", "x"): "api",
		api + "/":                      "api",
	} {
		p, err := s.ProjectAt(dir)
		if err != nil || p.Name != want {
			t.Fatalf("%s: got %q, %v; want %q", dir, p.Name, err, want)
		}
	}
	if _, err := s.ProjectAt(root + "/webx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sibling with shared prefix must not match, got %v", err)
	}
}

func TestProjectAtSeesThroughSymlinksAndRoot(t *testing.T) {
	s := open(t, t.TempDir())
	real := filepath.Join(t.TempDir(), "real")
	os.MkdirAll(filepath.Join(real, "src"), 0755)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	s.AddProject(Project{Name: "linked", Path: link})
	if p, err := s.ProjectAt(filepath.Join(real, "src")); err != nil || p.Name != "linked" {
		t.Fatalf("real path inside a project registered by symlink: got %q, %v", p.Name, err)
	}

	s.AddProject(Project{Name: "everything", Path: "/"})
	if p, err := s.ProjectAt(os.TempDir()); err != nil || p.Name != "everything" {
		t.Fatalf("project at / should contain everything: got %q, %v", p.Name, err)
	}
}

func TestNoteNamesTheBranchOfTheLastCommit(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, commit := gitRepo(t)
	commit(t0.Add(-day))
	git(t, dir, t0, "branch", "-M", "main")
	git(t, dir, t0, "checkout", "-q", "-b", "spike")
	gitCommit(t, dir, "spike.go", t0.Add(day))
	git(t, dir, t0, "checkout", "-q", "main") // left the newest work on another branch
	s.AddProject(Project{Name: "app", Path: dir})

	c.t = t0.Add(2 * day)
	if got := status(t, s).Note.Lines[2]; got != "Branch: spike" {
		t.Fatalf("got %q", got)
	}
}

func TestNoteNamesTheCheckedOutBranchWhenTheCommitIsOnSeveral(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, commit := gitRepo(t)
	commit(t0.Add(-day))
	git(t, dir, t0, "branch", "-M", "main")
	git(t, dir, t0, "branch", "a-old-feature") // sorts first and holds the same commit
	git(t, dir, t0, "checkout", "-q", "-b", "z-current")
	s.AddProject(Project{Name: "app", Path: dir})

	c.t = t0.Add(2 * day)
	if got := status(t, s).Note.Lines[2]; got != "Branch: z-current" {
		t.Fatalf("got %q", got)
	}
}

func TestProjectNamedFindsTheFirstExactMatch(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddProject(Project{Name: "web", Path: "/one"})
	s.AddProject(Project{Name: "web", Path: "/two"})
	if p, err := s.ProjectNamed("web"); err != nil || p.Path != "/one" {
		t.Fatalf("got %+v, %v", p, err)
	}
	if _, err := s.ProjectNamed("Web"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("names are exact: got %v", err)
	}
}
