package core

import (
	"errors"
	"testing"
)

// deliveries runs one Check and returns the names it delivered.
func deliveries(t *testing.T, s *Store, fail error) (got []string) {
	t.Helper()
	err := s.Check(func(due []Status) error {
		for _, st := range due {
			got = append(got, st.Name)
		}
		return fail
	})
	if !errors.Is(err, fail) {
		t.Fatalf("check: %v", err)
	}
	return got
}

func TestCheckNotifiesOncePerStalePeriod(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	p := onlyProject(t, s)

	c.t = t0.Add(6 * day)
	if got := deliveries(t, s, nil); len(got) != 0 {
		t.Fatalf("not stale yet: got %v", got)
	}
	c.t = t0.Add(8 * day)
	if got := deliveries(t, s, nil); len(got) != 1 {
		t.Fatalf("newly stale: got %v", got)
	}
	c.t = t0.Add(9 * day)
	if got := deliveries(t, s, nil); len(got) != 0 {
		t.Fatalf("same stale period: got %v", got)
	}

	s.Snooze(p.ID, 2*day) // ends at day 11: a new notification then
	c.t = t0.Add(10 * day)
	if got := deliveries(t, s, nil); len(got) != 0 {
		t.Fatalf("snoozed: got %v", got)
	}
	c.t = t0.Add(11*day + 1)
	if got := deliveries(t, s, nil); len(got) != 1 {
		t.Fatalf("snooze over, still stale: got %v", got)
	}

	c.t = t0.Add(12 * day)
	s.AddMilestone(p.ID, "back") // activity ends the period
	c.t = t0.Add(18 * day)
	if got := deliveries(t, s, nil); len(got) != 0 {
		t.Fatalf("active again: got %v", got)
	}
	c.t = t0.Add(20 * day)
	if got := deliveries(t, s, nil); len(got) != 1 {
		t.Fatalf("a new stale period: got %v", got)
	}
}

func TestFailedDeliveryIsRetried(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	c.t = t0.Add(8 * day)
	boom := errors.New("no notification daemon")
	deliveries(t, s, boom)
	if got := deliveries(t, s, nil); len(got) != 1 {
		t.Fatalf("failed delivery must be retried: got %v", got)
	}
}

func TestNotifiedStateSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	c := &clock{t0}
	s, _ := Open(dir, c.now)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	c.t = t0.Add(8 * day)
	deliveries(t, s, nil)
	s.Close()
	s, _ = Open(dir, c.now)
	defer s.Close()
	if got := deliveries(t, s, nil); len(got) != 0 {
		t.Fatalf("restart must not repeat: got %v", got)
	}
}

func TestModuleSwitches(t *testing.T) {
	s := open(t, t.TempDir())
	for m, want := range map[Module]bool{ModuleNotifications: true, ModuleTray: true, ModuleEmail: false, ModuleBrain: false} {
		if on, err := s.ModuleOn(m); err != nil || on != want {
			t.Errorf("%s default: got %v, %v", m, on, err)
		}
	}
	if err := s.SetModule(ModuleNotifications, false); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.ModuleOn(ModuleNotifications); on {
		t.Error("notifications should be off")
	}
	if err := s.SetModule("telepathy", true); err == nil {
		t.Error("unknown module should fail")
	}
}
