package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakePath makes PATH hold only fake executables with these names.
func fakePath(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("TERMINAL", "")
}

// project adds one project in a temp folder and returns it.
func project(t *testing.T, s *Store) Project {
	t.Helper()
	if err := s.AddProject(Project{Name: "web", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	ps, _ := s.Projects()
	return ps[0]
}

func launchArgs(t *testing.T, s *Store, p Project, program string, attached bool) []string {
	t.Helper()
	cmd, err := s.Launch(p, program, attached)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != p.Path {
		t.Fatalf("working folder %q, want the project's %q", cmd.Dir, p.Path)
	}
	return cmd.Args
}

func TestTerminalProgramOpensInTerminalWithoutOne(t *testing.T) {
	fakePath(t, "myterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)

	got := launchArgs(t, s, p, "nvim", false)
	if want := []string{"myterm", "-e", "nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTerminalProgramRunsInPlaceWithATerminal(t *testing.T) {
	fakePath(t, "myterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "nvim", true), []string{"nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestGUIProgramIsNeverWrapped(t *testing.T) {
	fakePath(t, "myterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "code", false), []string{"code", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTerminalSettingBeatsTERMINAL(t *testing.T) {
	fakePath(t, "myterm", "wezterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)
	if err := s.SetTerminal("wezterm"); err != nil {
		t.Fatal(err)
	}

	if got, want := launchArgs(t, s, p, "claude", false), []string{"wezterm", "start", "--", "claude", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// several words are used as written
	s.SetTerminal("myterm --hold -x")
	if got, want := launchArgs(t, s, p, "claude", false), []string{"myterm", "--hold", "-x", "claude", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// empty goes back to detecting
	s.SetTerminal("")
	if got, err := s.Terminal(); err != nil || got != "" {
		t.Fatalf("cleared setting: got %q, %v", got, err)
	}
	if got := launchArgs(t, s, p, "claude", false); got[0] != "myterm" || got[1] != "-e" {
		t.Fatalf("got %v, want $TERMINAL with -e", got)
	}
}

func TestMissingTERMINALFallsBackToAKnownTerminal(t *testing.T) {
	fakePath(t, "xterm")
	t.Setenv("TERMINAL", "gone")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "lazygit", false), []string{"xterm", "-e", "lazygit", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNoTerminalNamesTheSettingAndStillCountsAsActivity(t *testing.T) {
	fakePath(t)
	c := &clock{t0}
	s := openAt(t, c)
	p := project(t, s)

	c.t = t0.Add(5 * day)
	_, err := s.Launch(p, "nvim", false)
	if err == nil || !strings.Contains(err.Error(), "proz config terminal") {
		t.Fatalf("want an error naming the setting, got %v", err)
	}
	st, err := s.Status(p.ID)
	if err != nil || !st.LastActivity.Equal(t0.Add(5*day)) {
		t.Fatalf("want the launch recorded, got %v, %v", st.LastActivity, err)
	}
}
