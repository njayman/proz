package core

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestIdeaIsStaleOnRegistrationLaunchAndMilestoneOnly(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	repo, commit := gitRepo(t)
	t.Chdir(repo) // a repo in the working directory must never count for an idea
	if name, err := s.AddIdea("  habit tracker app \n"); err != nil || name != "habit tracker app" {
		t.Fatalf("got %q, %v", name, err)
	}
	idea := onlyProject(t, s)
	if idea.Name != "habit tracker app" || idea.Path != "" {
		t.Fatalf("got %+v", idea)
	}

	c.t = t0.Add(3 * day)
	s.AddMilestone(idea.ID, "sketched the data model")
	commit(t0.Add(10 * day))
	c.t = t0.Add(11 * day)
	ns := nudgesAt(t, s)
	if len(ns) != 1 || !ns[0].LastActivity.Equal(t0.Add(3*day)) {
		t.Fatalf("want idea stale since its milestone, got %+v", ns)
	}
	if _, err := s.Launch(idea, "", true); err != nil {
		t.Fatal(err)
	}
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("a launch is activity for an idea too, got %+v", ns)
	}
	if _, err := s.AddIdea(" "); err == nil {
		t.Fatal("blank idea should fail")
	}
}

func TestIdeaForMatchesAFolderNamedLikeTheIdea(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("Habit Tracker App")
	s.AddIdea("proz")
	s.AddProject(Project{Name: "web", Path: "/elsewhere/web"})

	for dir, want := range map[string]string{
		"/code/habit-tracker-app": "Habit Tracker App",
		"/code/habit_tracker_app": "Habit Tracker App",
		"/code/HabitTrackerApp":   "",
		"/code/Proz":              "proz",
		"/code/web":               "", // only ideas match, never projects that have a folder
	} {
		p, err := s.IdeaFor(dir)
		if want == "" {
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("%s: want no idea, got %q, %v", dir, p.Name, err)
			}
			continue
		}
		if err != nil || p.Name != want {
			t.Fatalf("%s: got %q, %v; want %q", dir, p.Name, err, want)
		}
	}

	// attaching is just giving the idea a folder; then it no longer matches
	p, _ := s.IdeaFor("/code/proz")
	p.Path = filepath.Join("/code", "proz")
	if err := s.UpdateProject(p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.IdeaFor("/code/proz"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("attached idea should not match again, got %v", err)
	}
}

func TestProjectNameCollapsesSpacesAndNeedsText(t *testing.T) {
	if got, err := ProjectName("  habit \n tracker "); err != nil || got != "habit tracker" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := ProjectName(" \t"); !errors.Is(err, ErrNoName) {
		t.Fatalf("blank: %v", err)
	}
	if _, err := open(t, t.TempDir()).AddIdea(" "); !errors.Is(err, ErrNoName) {
		t.Fatalf("a blank idea: %v", err)
	}
}

func TestNewProjectReturnsItWithItsID(t *testing.T) {
	s := open(t, t.TempDir())
	if err := s.AddProject(Project{Name: "first"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.NewProject(Project{Name: "second", Programs: []string{"code"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ProjectNamed("second")
	if err != nil || p.ID == 0 || got.ID != p.ID || p.Name != "second" {
		t.Fatalf("returned %+v, stored %+v, %v", p, got, err)
	}
}
