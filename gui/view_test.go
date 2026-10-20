package gui

import (
	"errors"
	"os"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/njayman/proz/core"
)

// TestMain keeps tests away from the real ~/.claude session logs.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "proz-claude")
	if err != nil {
		panic(err)
	}
	os.Setenv("CLAUDE_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

type opened struct{ name, folder, program string }

// launcher opens the view over a temp store holding projects, in a headless
// window. It returns the window, what the fake launch function was asked to
// open, and whether the window has closed.
func launcher(t *testing.T, projects ...core.Project) (fyne.Window, *[]opened, *bool) {
	t.Helper()
	return launcherFailing(t, nil, projects...)
}

// launcherFailing is launcher whose launch function returns fail.
func launcherFailing(t *testing.T, fail error, projects ...core.Project) (fyne.Window, *[]opened, *bool) {
	t.Helper()
	s, err := core.Open(t.TempDir(), func() time.Time { return t0 })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, p := range projects {
		if err := s.AddProject(p); err != nil {
			t.Fatal(err)
		}
	}
	var got []opened
	w := test.NewWindow(nil)
	closed := false
	w.SetOnClosed(func() { closed = true })
	t.Cleanup(func() {
		if !closed { // the test driver panics on a second close
			w.Close()
		}
	})
	New(w, s, func(p core.Project, folder, program string) error {
		got = append(got, opened{p.Name, folder, program})
		return fail
	})
	return w, &got, &closed
}

// press sends key where the driver would: the focused widget, else the canvas.
func press(w fyne.Window, key fyne.KeyName) {
	e := &fyne.KeyEvent{Name: key}
	if f := w.Canvas().Focused(); f != nil {
		f.TypedKey(e)
	} else if h := w.Canvas().OnTypedKey(); h != nil {
		h(e)
	}
}

func TestEnterOpensTheFirstProjectWithItsMainProgramAndCloses(t *testing.T) {
	w, got, closed := launcher(t,
		core.Project{Name: "web", Path: "/src/web", Programs: []string{"code", "zed"}},
		core.Project{Name: "api", Path: "/src/api", Programs: []string{"nvim"}})
	press(w, fyne.KeyReturn)

	if len(*got) != 1 || (*got)[0] != (opened{"web", "/src/web", "code"}) {
		t.Fatalf("opened %+v", *got)
	}
	if !*closed {
		t.Fatal("the window must close after a launch")
	}
}

func TestAProjectWithNoProgramsOpensATerminal(t *testing.T) {
	w, got, _ := launcher(t, core.Project{Name: "notes", Path: "/src/notes"})
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || (*got)[0] != (opened{"notes", "/src/notes", ""}) {
		t.Fatalf("opened %+v", *got)
	}
}

func TestEnterWithNoProjectsOpensNothing(t *testing.T) {
	w, got, closed := launcher(t)
	press(w, fyne.KeyReturn)
	if len(*got) != 0 || *closed {
		t.Fatalf("opened %+v, closed %v", *got, *closed)
	}
}

func TestAFailedLaunchKeepsTheWindowOpen(t *testing.T) {
	w, got, closed := launcherFailing(t, errors.New("code not found"),
		core.Project{Name: "web", Path: "/src/web", Programs: []string{"code"}})
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || *closed {
		t.Fatalf("opened %+v, closed %v", *got, *closed)
	}
}
