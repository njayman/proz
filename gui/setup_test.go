package gui

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/njayman/proz/core"
)

// launcherWith is launcher over a store the test can read: it returns the
// window, the store, what was opened and whether the window closed.
func launcherWith(t *testing.T, projects ...core.Project) (fyne.Window, *core.Store, *[]opened, *bool) {
	t.Helper()
	slowStatuses(t)
	var s *core.Store
	now := t0
	w, got, closed := launcherOver(t, nil, &now, func(st *core.Store) {
		s = st
		for _, p := range projects {
			must(t, s.AddProject(p))
		}
	})
	return w, s, got, closed
}

// named is the project in s named name.
func named(t *testing.T, s *core.Store, name string) core.Project {
	t.Helper()
	p, err := s.ProjectNamed(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return p
}

func TestCtrlNOrNewOpensSetupInPlaceAndSaveAddsTheIdeaSelected(t *testing.T) {
	w, s, got, _ := launcherWith(t, core.Project{Name: "web", Path: "/src/web"}, core.Project{Name: "older idea"}) // lists above the new one
	shortcut(w, fyne.KeyN)
	if !showing(w, "Name", "Programs") || showing(w, "web") {
		t.Fatalf("Ctrl+N shows setup in place of home: %q", texts(w))
	}
	if f, ok := w.Canvas().Focused().(*escEntry); !ok || f.PlaceHolder != "Name" {
		t.Fatalf("setup opens with the keys in Name, not on %T", w.Canvas().Focused())
	}
	typeText(w, "habit   tracker")
	test.Type(entry(t, w, "Such as code or nvim"), "code")
	test.Tap(button(t, w, "Save"))

	p := named(t, s, "habit tracker")
	if p.Path != "" || !slices.Equal(p.Programs, []string{"code"}) {
		t.Fatalf("saved %+v", p)
	}
	if !showing(w, "web", "habit tracker") {
		t.Fatalf("Save returns home with the new one listed: %q", texts(w))
	}
	press(w, fyne.KeyReturn) // the new idea is selected: Enter sets it up
	if len(*got) != 0 || entry(t, w, "Name").Text != "habit tracker" {
		t.Fatalf("opened %+v, screen %q", *got, texts(w))
	}

	press(w, fyne.KeyEscape)
	test.Tap(button(t, w, "New…"))
	if e := entry(t, w, "Name"); e.Text != "" {
		t.Fatalf("New… starts empty: %q", e.Text)
	}
	if size := w.Canvas().Size(); size != fyne.NewSize(720, 480) {
		t.Fatalf("the window resized to %v", size)
	}
}

func TestSetUpEditsTheSelectedProjectsNameAndProgramsAndKeepsTheRest(t *testing.T) {
	w, s, got, _ := launcherWith(t, core.Project{Name: "web", Path: "/src/web", Programs: []string{"code"}, Arguments: []string{"--new-window"}})
	test.Tap(button(t, w, "Set up…"))
	name, programs := entry(t, w, "Name"), entry(t, w, "Such as code or nvim")
	if name.Text != "web" || programs.Text != "code" {
		t.Fatalf("shows what is saved: %q, %q", name.Text, programs.Text)
	}
	name.SetText("website")
	programs.SetText("zed code")
	test.Tap(button(t, w, "Save"))

	p := named(t, s, "website")
	if p.Path != "/src/web" || !slices.Equal(p.Programs, []string{"zed", "code"}) || !slices.Equal(p.Arguments, []string{"--new-window"}) {
		t.Fatalf("saved %+v", p)
	}
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || (*got)[0] != (opened{"website", "/src/web", Way{Program: "zed"}}) {
		t.Fatalf("home selects it, with its new main program: %+v", *got)
	}
}

func TestCancelOrEscLeavesSetupWithoutChanges(t *testing.T) {
	w, s, _, closed := launcherWith(t, core.Project{Name: "web", Path: "/src/web"})
	for _, leave := range []func(){
		func() { test.Tap(button(t, w, "Cancel")) },
		func() { press(w, fyne.KeyEscape) },
	} {
		test.Tap(button(t, w, "Set up…"))
		w.Canvas().Focus(entry(t, w, "Name"))
		entry(t, w, "Name").SetText("renamed")
		leave()
		if *closed || !showing(w, "web") {
			t.Fatalf("back home: closed %v, %q", *closed, texts(w))
		}
		named(t, s, "web")
	}
}

func TestSetupNeedsANameAndStaysOpenUntilItHasOne(t *testing.T) {
	w, s, _, _ := launcherWith(t)
	shortcut(w, fyne.KeyN)
	test.Type(entry(t, w, "Name"), "   ")
	test.Tap(button(t, w, "Save"))
	if ps, _ := s.Projects(); len(ps) != 0 || !says(w, "needs a name") {
		t.Fatalf("saved %+v, screen %q", ps, texts(w))
	}
}

func TestAnIdeaHasSetUpInPlaceOfOpenButtonsAndEnterOpensIt(t *testing.T) {
	w, _, got, closed := launcherWith(t, core.Project{Name: "someday"})
	if !button(t, w, "Set up…").Visible() || slices.ContainsFunc(objects(w), func(o fyne.CanvasObject) bool { _, ok := o.(*widget.Select); return ok }) {
		t.Fatal("an idea has Set up… and no ways to open")
	}
	press(w, fyne.KeyReturn)
	if len(*got) != 0 || *closed || entry(t, w, "Name").Text != "someday" {
		t.Fatalf("Enter on an idea opens its setup: opened %+v, closed %v, %q", *got, *closed, texts(w))
	}
}
