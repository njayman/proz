//go:build !windows

package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// slowGit puts a git on PATH that starts a child and hangs, writing the
// child's pid to a file in pids, and shortens the git limit.
func slowGit(t *testing.T) (pids string) {
	t.Helper()
	bin, pids := t.TempDir(), t.TempDir()
	script := "#!/bin/sh\nsleep 30 &\necho $! > " + pids + "/$$\nwait\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	was := gitLimit
	gitLimit = 300 * time.Millisecond
	t.Cleanup(func() { gitLimit = was })
	return pids
}

func TestSlowGitStillReturnsTheOtherSignals(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	slowGit(t)
	s.AddProject(Project{Name: "lfs", Path: t.TempDir()})
	p := onlyProject(t, s)
	c.t = t0.Add(day)
	s.AddMilestone(p.ID, "spike done")
	c.t = t0.Add(3 * day)

	start := time.Now()
	st, err := s.Status(p.ID)
	if took := time.Since(start); took > time.Second {
		t.Fatalf("took %v, want about the 300ms limit", took)
	}
	if err != nil || !st.LastActivity.Equal(t0.Add(day)) || !strings.Contains(st.Note.Full(), "Milestone: spike done") {
		t.Fatalf("want the milestone signal, got %+v, %v", st, err)
	}
}

func TestSlowGitLeavesNoProcessBehind(t *testing.T) {
	s := open(t, t.TempDir())
	pids := slowGit(t)
	s.AddProject(Project{Name: "lfs", Path: t.TempDir()})

	if _, err := s.Statuses(); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(pids)
	if len(files) == 0 {
		t.Fatal("the fake git never ran")
	}
	time.Sleep(100 * time.Millisecond) // let the kill land
	for _, f := range files {
		b, _ := os.ReadFile(filepath.Join(pids, f.Name()))
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 && syscall.Kill(pid, 0) == nil {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Errorf("git's child %d is still running", pid)
		}
	}
}

func TestSlowProjectsDoNotAddUp(t *testing.T) {
	s := open(t, t.TempDir())
	slowGit(t)
	for _, name := range []string{"a", "b", "c"} {
		s.AddProject(Project{Name: name, Path: t.TempDir()})
	}

	start := time.Now()
	all, err := s.Statuses()
	if took := time.Since(start); err != nil || len(all) != 3 || took > 550*time.Millisecond {
		t.Fatalf("three slow projects took %v (%d, %v), want under twice the 300ms limit", took, len(all), err)
	}
}
