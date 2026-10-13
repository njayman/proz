package core

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

const day = 24 * time.Hour

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

func nudgesAt(t *testing.T, s *Store) []Nudge {
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
	return dir, func(at time.Time) {
		d := strconv.FormatInt(at.Unix(), 10) + " +0000"
		run([]string{"GIT_AUTHOR_DATE=" + d, "GIT_COMMITTER_DATE=" + d,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"},
			"commit", "-q", "--allow-empty", "-m", "work")
	}
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
	s.AddProject(Project{Name: "slow", Path: dir})
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
