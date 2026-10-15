package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func brainAt(t *testing.T, c *clock) (*Store, string) {
	t.Helper()
	s := openAt(t, c)
	brain := t.TempDir()
	if err := s.SetBrainPath(brain); err != nil {
		t.Fatal(err)
	}
	return s, brain
}

func TestNoBrainConfiguredStillWorks(t *testing.T) {
	s := openAt(t, &clock{t0})
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	if on, _ := s.ModuleOn(ModuleBrain); on {
		t.Fatal("brain is off by default")
	}
	if err := s.SetModule(ModuleBrain, true); err == nil {
		t.Fatal("brain stays off until a folder is set")
	}
	if err := s.SyncBrain(); err != nil {
		t.Fatalf("sync without a brain is a no-op: %v", err)
	}
	if _, _, err := s.BrainNote(onlyProject(t, s).ID); !errors.Is(err, ErrNoBrain) {
		t.Fatalf("read without a brain: %v", err)
	}
	if err := s.SetBrainPath(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a brain folder must exist")
	}
}

func TestBrainNoteHasMilestonesAndLeftOffNote(t *testing.T) {
	c := &clock{t0}
	s, brain := brainAt(t, c)
	if on, _ := s.ModuleOn(ModuleBrain); !on {
		t.Fatal("setting a brain folder switches it on")
	}
	s.AddProject(Project{Name: "Habit Tracker", Path: t.TempDir()})
	p := onlyProject(t, s)
	s.AddMilestone(p.ID, "shipped login; next: signup")
	c.t = t0.Add(3 * day)
	if err := s.SyncBrain(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(brain, "habit-tracker.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!-- proz:start -->", "<!-- proz:end -->", "Quiet for 3 days", "Milestone: shipped login; next: signup", "- " + t0.Format("2006-01-02") + ": shipped login; next: signup"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("note missing %q:\n%s", want, data)
		}
	}
	path, content, err := s.BrainNote(p.ID)
	if err != nil || path != filepath.Join(brain, "habit-tracker.md") || content != string(data) {
		t.Fatalf("read: %q, %v", path, err)
	}
}

func TestRewritesPreserveTextOutsideTheProzSection(t *testing.T) {
	c := &clock{t0}
	s, brain := brainAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)
	note := filepath.Join(brain, "app.md")
	os.WriteFile(note, []byte("# App\n\nmy own notes\n"), 0o644)

	s.SyncBrain() // an existing note gets the section appended
	data, _ := os.ReadFile(note)
	if !strings.HasPrefix(string(data), "# App\n\nmy own notes\n") {
		t.Fatalf("user text lost:\n%s", data)
	}

	edited := "# App\n\nmy own notes\n" + string(data)[len("# App\n\nmy own notes\n"):] + "\nmore notes below\n"
	edited = strings.Replace(edited, "<!-- proz:start -->", "above the section\n<!-- proz:start -->", 1)
	os.WriteFile(note, []byte(edited), 0o644)
	s.AddMilestone(p.ID, "second")
	c.t = t0.Add(day)
	s.SyncBrain()
	data, _ = os.ReadFile(note)
	got := string(data)
	for _, want := range []string{"my own notes\n", "above the section\n<!-- proz:start -->", "<!-- proz:end -->\n\nmore notes below\n", "Milestone: second"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "<!-- proz:start -->") != 1 {
		t.Errorf("one section only:\n%s", got)
	}
	if fi, _ := os.Stat(note); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode changed to %v", fi.Mode().Perm())
	}
}

func TestBrokenMarkersNeverLoseUserText(t *testing.T) {
	s, brain := brainAt(t, &clock{t0})
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	note := filepath.Join(brain, "app.md")
	// the end marker was deleted by hand: everything after the start is the user's
	os.WriteFile(note, []byte("<!-- proz:start -->\nold\nkeep me\n"), 0o644)
	s.SyncBrain()
	data, _ := os.ReadFile(note)
	if !strings.Contains(string(data), "keep me") || !strings.Contains(string(data), "<!-- proz:end -->") {
		t.Fatalf("got:\n%s", data)
	}
}

func TestNoteKeepsItsFileAcrossRenamesAndDuplicatesGetTheirOwn(t *testing.T) {
	s, brain := brainAt(t, &clock{t0})
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	s.SyncBrain()
	ps, _ := s.Projects()
	a, _, _ := s.BrainNote(ps[0].ID)
	b, _, _ := s.BrainNote(ps[1].ID)
	if a == b || filepath.Dir(a) != brain {
		t.Fatalf("duplicates share a note: %q %q", a, b)
	}
	ps[0].Name = "renamed"
	s.UpdateProject(ps[0])
	if again, _, _ := s.BrainNote(ps[0].ID); again != a {
		t.Fatalf("rename moved the note: %q -> %q", a, again)
	}
}

func TestSyncWritesOnlyWhenTheSectionChanges(t *testing.T) {
	c := &clock{t0}
	s, brain := brainAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	s.SyncBrain()
	note := filepath.Join(brain, "app.md")
	old := time.Unix(1, 0)
	os.Chtimes(note, old, old)
	s.SyncBrain()
	if fi, _ := os.Stat(note); !fi.ModTime().Equal(old) {
		t.Fatal("unchanged note was rewritten")
	}
}

func TestMarkerTextInsideContentCannotBreakTheSection(t *testing.T) {
	s, brain := brainAt(t, &clock{t0})
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	s.AddMilestone(onlyProject(t, s).ID, "fixed <!-- proz:end --> parsing")
	s.SyncBrain()
	data, _ := os.ReadFile(filepath.Join(brain, "app.md"))
	if strings.Count(string(data), "<!-- proz:end -->") != 1 {
		t.Fatalf("got:\n%s", data)
	}
}
