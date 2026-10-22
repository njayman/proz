package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const (
	day  = 24 * time.Hour
	hour = time.Hour
)

// clock is a settable fake clock.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func openAt(t *testing.T, c *clock) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), c.now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func nudgesAt(t *testing.T, s *Store) []Status {
	t.Helper()
	ns, err := s.Nudges()
	if err != nil {
		t.Fatal(err)
	}
	return ns
}

func TestNewProjectGoesStaleAfterThreshold(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "idea", Path: t.TempDir()})

	c.t = t0.Add(7 * day)
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("at exactly 7 days want none, got %+v", ns)
	}
	c.t = t0.Add(8 * day)
	ns := nudgesAt(t, s)
	if len(ns) != 1 || ns[0].Name != "idea" || !ns[0].LastActivity.Equal(t0) || ns[0].Quiet != 8*day {
		t.Fatalf("got %+v", ns)
	}
}

// gitRepo makes a real repo; commit adds a commit dated at.
func gitRepo(t *testing.T) (dir string, commit func(at time.Time)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed; skipping git activity test")
	}
	dir = t.TempDir()
	run := func(env []string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(nil, "init", "-q")
	return dir, func(at time.Time) { gitCommit(t, dir, "work.txt", at) }
}

var gitEnv = []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}

// git runs git in dir with commit dates pinned to at.
func git(t *testing.T, dir string, at time.Time, args ...string) {
	t.Helper()
	d := strconv.FormatInt(at.Unix(), 10) + " +0000"
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), gitEnv...), "GIT_AUTHOR_DATE="+d, "GIT_COMMITTER_DATE="+d)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitCommit changes file (relative to dir) and commits it, dated at.
func gitCommit(t *testing.T, dir, file string, at time.Time) {
	t.Helper()
	path := filepath.Join(dir, file)
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(at.String()), 0644)
	git(t, dir, at, "add", "-A")
	git(t, dir, at, "commit", "-q", "-m", "work")
}

func TestCommitIsActivityAndEndsStalePeriod(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, commit := gitRepo(t)
	commit(t0.Add(-30 * day)) // older than registration: registration wins
	s.AddProject(Project{Name: "app", Path: dir})
	commit(t0.Add(5 * day))

	c.t = t0.Add(10 * day)
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("commit 5 days ago: want none, got %+v", ns)
	}
	c.t = t0.Add(13 * day)
	ns := nudgesAt(t, s)
	if len(ns) != 1 || !ns[0].LastActivity.Equal(t0.Add(5*day)) || ns[0].Quiet != 8*day {
		t.Fatalf("got %+v", ns)
	}
	commit(t0.Add(12 * day))
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("new commit should end stale period, got %+v", ns)
	}
}

func TestStaleThresholdIsConfigurableAndPersists(t *testing.T) {
	dir := t.TempDir()
	c := &clock{t0}
	s, _ := Open(dir, c.now)
	if d, _ := s.StaleThreshold(); d != 7*day {
		t.Fatalf("default want 7 days, got %v", d)
	}
	s.AddProject(Project{Name: "slow", Path: t.TempDir()}) // not the store dir: its database file is activity
	if err := s.SetStaleThreshold(14 * day); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, _ = Open(dir, c.now)
	defer s.Close()
	c.t = t0.Add(10 * day)
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("threshold 14 days: want none at 10, got %+v", ns)
	}
	c.t = t0.Add(15 * day)
	if ns := nudgesAt(t, s); len(ns) != 1 {
		t.Fatalf("want stale at 15 days, got %+v", ns)
	}
	if err := s.SetStaleThreshold(0); err == nil {
		t.Fatal("zero threshold should be rejected")
	}
}

func TestOnlyTheUsersOwnWorkInTheProjectFolderCounts(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	repo, commit := gitRepo(t)
	commit(t0.Add(-day))
	sub := filepath.Join(repo, "app")
	gitCommit(t, repo, "app/main.go", t0.Add(-day))
	s.AddProject(Project{Name: "app", Path: sub})

	gitCommit(t, repo, "other/x.go", t0.Add(9*day)) // elsewhere in the repo
	os.WriteFile(filepath.Join(sub, "main.go"), []byte("wip"), 0644)
	git(t, repo, t0.Add(9*day), "stash", "-q")                                    // a stash is not a commit
	git(t, repo, t0.Add(9*day), "update-ref", "refs/remotes/origin/main", "HEAD") // fetched work is someone else's

	c.t = t0.Add(10 * day)
	ns := nudgesAt(t, s)
	if len(ns) != 1 || !ns[0].LastActivity.Equal(t0) {
		t.Fatalf("want stale since registration, got %+v", ns)
	}
}

func TestProjectWithoutFolderIgnoresCurrentDirectoryRepo(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	repo, commit := gitRepo(t)
	commit(t0.Add(9 * day))
	t.Chdir(repo)
	s.AddProject(Project{Name: "idea"})

	c.t = t0.Add(10 * day)
	if ns := nudgesAt(t, s); len(ns) != 1 {
		t.Fatalf("want stale idea, got %+v", ns)
	}
}

func onlyProject(t *testing.T, s *Store) Project {
	t.Helper()
	ps, err := s.Projects()
	if err != nil || len(ps) != 1 {
		t.Fatalf("want one project, got %+v, %v", ps, err)
	}
	return ps[0]
}

func TestLastActivityIsNewestOfRegistrationLaunchAndCommit(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, commit := gitRepo(t)
	s.AddProject(Project{Name: "app", Path: dir})
	p := onlyProject(t, s)

	c.t = t0.Add(5 * day)
	if _, err := s.Launch(p, "", "", true); err != nil { // opening without a program still counts
		t.Fatal(err)
	}
	c.t = t0.Add(10 * day)
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("launched 5 days ago: want none, got %+v", ns)
	}
	c.t = t0.Add(13 * day)
	if ns := nudgesAt(t, s); len(ns) != 1 || !ns[0].LastActivity.Equal(t0.Add(5*day)) {
		t.Fatalf("want stale since launch, got %+v", ns)
	}

	commit(t0.Add(4 * day)) // older than the launch: launch still wins
	if ns := nudgesAt(t, s); len(ns) != 1 || !ns[0].LastActivity.Equal(t0.Add(5*day)) {
		t.Fatalf("older commit must not win, got %+v", ns)
	}
	commit(t0.Add(12 * day))
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("newer commit wins, want none, got %+v", ns)
	}
}

func TestProjectThresholdOverridesGlobalUntilCleared(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "slow", Path: t.TempDir()})
	s.AddProject(Project{Name: "fast", Path: t.TempDir()})
	ps, _ := s.Projects()
	if err := s.SetProjectStaleThreshold(ps[0].ID, 30*day); err != nil {
		t.Fatal(err)
	}

	c.t = t0.Add(10 * day)
	if ns := nudgesAt(t, s); len(ns) != 1 || ns[0].Name != "fast" {
		t.Fatalf("slow has 30 days: want only fast, got %+v", ns)
	}
	if err := s.SetProjectStaleThreshold(ps[0].ID, 0); err != nil { // 0 clears the override
		t.Fatal(err)
	}
	if ns := nudgesAt(t, s); len(ns) != 2 {
		t.Fatalf("cleared override falls back to global: want both, got %+v", ns)
	}
	if err := s.SetProjectStaleThreshold(42, 30*day); err == nil {
		t.Fatal("override on unknown project should fail")
	}
}
