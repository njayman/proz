package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// digests runs one Digest and returns the names it delivered, or nil if it
// delivered nothing.
func digests(t *testing.T, s *Store, fail error) (got []string) {
	t.Helper()
	err := s.Digest(func(stale []Status) error {
		if fail != nil {
			return fail
		}
		for _, st := range stale {
			got = append(got, st.Name)
		}
		return nil
	})
	if !errors.Is(err, fail) {
		t.Fatalf("digest: %v", err)
	}
	return got
}

// t0 is a Thursday; the default digest day is Monday.
var monday = time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)

func TestDigestOncePerWeekOnTheDigestDay(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	s.AddProject(Project{Name: "fresh", Path: t.TempDir()})

	c.t = monday.Add(-24 * time.Hour) // Sunday: last Monday's digest, caught up
	digests(t, s, nil)
	c.t = monday
	if p, _ := s.ProjectNamed("fresh"); p.ID != 0 {
		s.AddMilestone(p.ID, "busy")
	}
	if got := digests(t, s, nil); len(got) != 1 || got[0] != "app" {
		t.Fatalf("monday: got %v", got)
	}
	c.t = monday.Add(26 * time.Hour)
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("same week: got %v", got)
	}
	c.t = monday.Add(8 * 24 * time.Hour) // the next week; fresh is quiet again too
	if got := digests(t, s, nil); len(got) != 2 {
		t.Fatalf("next week: got %v", got)
	}
}

func TestDigestCatchesUpAfterTheDayAndSkipsQuietWeeks(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})

	c.t = monday.Add(-7 * 24 * time.Hour) // the Monday after t0: nothing stale
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("nothing stale: got %v", got)
	}
	c.t = monday.Add(-24 * time.Hour) // same week, now stale: the week was already done
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("quiet week stays quiet: got %v", got)
	}
	c.t = monday.Add(2 * 24 * time.Hour) // machine was off on Monday
	if got := digests(t, s, nil); len(got) != 1 {
		t.Fatalf("catch up on wednesday: got %v", got)
	}
}

func TestDigestDayIsConfigurable(t *testing.T) {
	c := &clock{monday} // registered Monday, stale by the next Tuesday
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	if d, err := s.DigestDay(); err != nil || d != time.Monday {
		t.Fatalf("default: %v, %v", d, err)
	}
	if err := s.SetDigestDay(time.Friday); err != nil {
		t.Fatal(err)
	}
	c.t = monday.Add(7 * 24 * time.Hour) // Monday: last Friday's week, nothing stale yet
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("monday: got %v", got)
	}
	c.t = monday.Add(11 * 24 * time.Hour) // Friday
	if got := digests(t, s, nil); len(got) != 1 {
		t.Fatalf("friday: got %v", got)
	}
}

func TestFailedDigestIsRetriedAndSentStateSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	c := &clock{t0}
	s, _ := Open(dir, c.now)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	c.t = monday
	boom := errors.New("smtp down")
	digests(t, s, boom)
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("a failure backs off before retrying: got %v", got)
	}
	c.t = monday.Add(time.Hour)
	if got := digests(t, s, nil); len(got) != 1 {
		t.Fatalf("retry after failure: got %v", got)
	}
	s.Close()
	s, _ = Open(dir, c.now)
	defer s.Close()
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("restart must not resend: got %v", got)
	}
}

func TestSMTPConfigIsOwnerOnlyAndEnablesEmail(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	if _, ok, err := s.SMTP(); ok || err != nil {
		t.Fatalf("unconfigured: %v, %v", ok, err)
	}
	if err := s.SetModule(ModuleEmail, true); err == nil {
		t.Fatal("email must stay off until SMTP is configured")
	}
	if err := s.SetSMTP(SMTP{Host: "smtp.example.com", From: "not an address", To: "me@example.com"}); err == nil {
		t.Fatal("bad from address should fail")
	}
	want := SMTP{Host: "smtp.example.com", Port: 587, Username: "me", Password: "hunter2", From: "proz <me@example.com>", To: "me@example.com"}
	if err := s.SetSMTP(want); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "smtp.json"))
	if err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) { // Windows has no Unix modes; the profile's ACL guards it
		t.Fatalf("smtp.json must be owner-only: %v, %v", fi, err)
	}
	if got, ok, err := s.SMTP(); !ok || err != nil || got != want {
		t.Fatalf("read back: %+v, %v, %v", got, ok, err)
	}
	if err := s.SetModule(ModuleEmail, true); err != nil {
		t.Fatalf("configured: %v", err)
	}
}

func TestSaveSMTPSwitchesEmailOnOnceAndKeepsThePasswordOnItsServer(t *testing.T) {
	s := open(t, t.TempDir())
	pw := "hunter2"
	c := SMTP{Host: "smtp.example.com", Port: DefaultSMTPPort, Username: "me", From: "me@example.com", To: "me@example.com"}
	if err := s.SaveSMTP(c, &pw); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.ModuleOn(ModuleEmail); !on {
		t.Fatal("the first mail server switches email on")
	}
	if err := s.SetModule(ModuleEmail, false); err != nil {
		t.Fatal(err)
	}
	c.To = "other@example.com"
	if err := s.SaveSMTP(c, nil); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.SMTP(); got.Password != pw || got.To != c.To {
		t.Fatalf("an edit keeps the saved password: %+v", got)
	}
	if on, _ := s.ModuleOn(ModuleEmail); on {
		t.Fatal("an edit keeps email as it was")
	}
	for _, moved := range []SMTP{{Host: "evil.example.com"}, {Username: "evil"}} {
		m := c
		if moved.Host != "" {
			m.Host = moved.Host
		} else {
			m.Username = moved.Username
		}
		if err := s.SaveSMTP(m, nil); !errors.Is(err, ErrPasswordAgain) {
			t.Fatalf("%+v with the saved password: %v", moved, err)
		}
	}
	if got, _, _ := s.SMTP(); got.Host != c.Host || got.Username != c.Username {
		t.Fatalf("a refused save changes nothing: %+v", got)
	}
	other := "other"
	c.Host = "new.example.com"
	if err := s.SaveSMTP(c, &other); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.SMTP(); got.Host != c.Host || got.Password != other {
		t.Fatalf("a new host with its password: %+v", got)
	}
}

func TestEmailWithNoSMTPSaysSo(t *testing.T) {
	s := open(t, t.TempDir())
	if err := s.SetModule(ModuleEmail, true); !errors.Is(err, ErrNoSMTP) {
		t.Fatalf("got %v", err)
	}
}

func TestFirstDigestWaitsForTheDigestDay(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	c.t = monday.Add(2 * 24 * time.Hour) // email set up on a Wednesday: no digest yet
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("wednesday, never sent: got %v", got)
	}
	c.t = monday.Add(7 * 24 * time.Hour)
	if got := digests(t, s, nil); len(got) != 1 {
		t.Fatalf("next monday: got %v", got)
	}
}

func TestChangingTheDigestDayNeverSendsTwiceInAWeek(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddProject(Project{Name: "app", Path: t.TempDir()})
	c.t = monday
	digests(t, s, nil)
	c.t = monday.Add(24 * time.Hour)
	s.SetDigestDay(time.Tuesday)
	if got := digests(t, s, nil); got != nil {
		t.Fatalf("sent yesterday: got %v", got)
	}
	c.t = monday.Add(8 * 24 * time.Hour) // the next Tuesday
	if got := digests(t, s, nil); len(got) != 1 {
		t.Fatalf("next tuesday: got %v", got)
	}
}

func TestLooseSMTPFileIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix permissions")
	}
	dir := t.TempDir()
	s := open(t, dir)
	s.SetSMTP(SMTP{Host: "h", Port: 587, Password: "pw", From: "a@b.c", To: "a@b.c"})
	os.Chmod(filepath.Join(dir, "smtp.json"), 0o644)
	if _, _, err := s.SMTP(); err == nil {
		t.Fatal("a password file others can read must be refused")
	}
}
