package core

import (
	"errors"
	"reflect"
	"testing"
)

func TestMilestoneIsActivityAndLeadsTheNote(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	dir, commit := gitRepo(t)
	commit(t0)
	git(t, dir, t0, "branch", "-M", "main")
	s.AddProject(Project{Name: "app", Path: dir})
	p := onlyProject(t, s)

	c.t = t0.Add(5 * day)
	if err := s.AddMilestone(p.ID, "  shipped login; next: signup\n"); err != nil {
		t.Fatal(err)
	}
	c.t = t0.Add(9 * day)
	st := status(t, s)
	want := []string{"Quiet for 4 days", "Milestone: shipped login; next: signup", "Last commit: work", "Branch: main"}
	if !reflect.DeepEqual(st.Note.Lines, want) || st.Stale {
		t.Fatalf("got %q stale=%v, want %q", st.Note.Lines, st.Stale, want)
	}
	if !st.LastActivity.Equal(t0.Add(5 * day)) {
		t.Fatalf("milestone should be the last activity, got %v", st.LastActivity)
	}
}

func TestMilestonesNewestFirstAndOnlyTheLatestInTheNote(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)

	s.AddMilestone(p.ID, "spike done")
	c.t = t0.Add(day)
	s.AddMilestone(p.ID, "api wired\nnext: tests") // multi-line keeps the note one line per signal

	ms, err := s.Milestones(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Text != "api wired\nnext: tests" || !ms[0].At.Equal(t0.Add(day)) || ms[1].Text != "spike done" {
		t.Fatalf("got %+v", ms)
	}
	if got := status(t, s).Note.Lines; len(got) != 2 || got[1] != "Milestone: api wired next: tests" {
		t.Fatalf("got %q", got)
	}
}

func TestMilestoneRejectsBlankTextAndUnknownProject(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddProject(Project{Name: "app", Path: "/app"})
	p := onlyProject(t, s)
	if err := s.AddMilestone(p.ID, " \n "); err == nil {
		t.Fatal("blank milestone should fail")
	}
	if err := s.AddMilestone(42, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown project: got %v", err)
	}
}

func TestRemovingAProjectRemovesItsMilestones(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddProject(Project{Name: "app", Path: "/app"})
	p := onlyProject(t, s)
	s.AddMilestone(p.ID, "x")
	if err := s.RemoveProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if ms, err := s.Milestones(p.ID); err != nil || len(ms) != 0 {
		t.Fatalf("got %+v, %v", ms, err)
	}
}
