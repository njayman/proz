package core

import (
	"errors"
	"testing"
	"time"
)

func stateAt(t *testing.T, s *Store, id int64) State {
	t.Helper()
	st, err := s.Status(id)
	if err != nil {
		t.Fatal(err)
	}
	return st.State
}

func TestSnoozeHidesTheNudgeUntilItEnds(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)

	c.t = t0.Add(10 * day)
	if got := stateAt(t, s, p.ID); got != StateStale {
		t.Fatalf("got %q", got)
	}
	if err := s.Snooze(p.ID, 3*day); err != nil {
		t.Fatal(err)
	}
	c.t = t0.Add(12 * day)
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("snoozed: want none, got %+v", ns)
	}
	st, _ := s.Status(p.ID)
	if st.State != StateSnoozed || !st.SnoozedUntil.Equal(t0.Add(13*day)) {
		t.Fatalf("got %q until %v", st.State, st.SnoozedUntil)
	}
	c.t = t0.Add(13*day + 1)
	if ns := nudgesAt(t, s); len(ns) != 1 {
		t.Fatalf("snooze over and still stale: want a nudge again, got %+v", ns)
	}
}

func TestDropArchivesUntilNewActivity(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)

	c.t = t0.Add(10 * day)
	if err := s.Drop(p.ID); err != nil {
		t.Fatal(err)
	}
	c.t = t0.Add(60 * day)
	if ns := nudgesAt(t, s); len(ns) != 0 {
		t.Fatalf("dropped: want none, got %+v", ns)
	}
	if got := archivedNames(t, s); len(got) != 1 || got[0] != "app" {
		t.Fatalf("archived: got %v", got)
	}

	s.AddMilestone(p.ID, "picked it back up") // any activity revives it
	if got := stateAt(t, s, p.ID); got != StateActive {
		t.Fatalf("after activity: got %q", got)
	}
	if got := archivedNames(t, s); len(got) != 0 {
		t.Fatalf("archived after activity: got %v", got)
	}
	c.t = t0.Add(68 * day)
	if ns := nudgesAt(t, s); len(ns) != 1 {
		t.Fatalf("revived project nudges again when quiet, got %+v", ns)
	}
}

func TestRestoreBringsADroppedProjectBack(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)
	c.t = t0.Add(10 * day)
	s.Snooze(p.ID, 30*day)
	s.Drop(p.ID)

	if err := s.Restore(p.ID); err != nil {
		t.Fatal(err)
	}
	if got := stateAt(t, s, p.ID); got != StateStale {
		t.Fatalf("restored quiet project is stale again (snooze cleared too), got %q", got)
	}
}

func TestRespondingToAnUnknownProjectFails(t *testing.T) {
	s := open(t, t.TempDir())
	for name, err := range map[string]error{
		"snooze":  s.Snooze(42, day),
		"drop":    s.Drop(42),
		"restore": s.Restore(42),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	s.AddProject(Project{Name: "app", Path: "/app"})
	if err := s.Snooze(onlyProject(t, s).ID, 0); err == nil {
		t.Error("zero snooze should fail")
	}
}

func archivedNames(t *testing.T, s *Store) (out []string) {
	t.Helper()
	as, err := s.Archived()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}

func TestNewActivityEndsTheSnoozeWithItsStalePeriod(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)

	c.t = t0.Add(10 * day)
	s.Snooze(p.ID, 30*day)
	c.t = t0.Add(12 * day)
	s.AddMilestone(p.ID, "back on it")
	c.t = t0.Add(20 * day) // quiet again, a new stale period, still inside the old snooze
	if ns := nudgesAt(t, s); len(ns) != 1 {
		t.Fatalf("a new stale period must nudge despite the old snooze, got %+v", ns)
	}
}

func TestSnoozeUntilATimeNudgesAgainThen(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)
	c.t = t0.Add(10 * day)
	until := t0.Add(20 * day)
	for name, bad := range map[string]time.Time{"now": c.t, "past": t0, "over 100 years": c.t.Add(101 * 365 * day)} {
		if err := s.SnoozeUntil(p.ID, bad); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if st, _ := s.Status(p.ID); st.State != StateStale {
		t.Fatalf("a refused snooze changes nothing: %v", st.State)
	}
	if err := s.SnoozeUntil(p.ID, until); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Status(p.ID); st.State != StateSnoozed || !st.SnoozedUntil.Equal(until) {
		t.Fatalf("got %v until %v", st.State, st.SnoozedUntil)
	}
	c.t = until
	if st, _ := s.Status(p.ID); st.State != StateStale {
		t.Fatalf("nudges again when it ends: %v", st.State)
	}
}
