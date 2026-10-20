package gui

import (
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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

type opened struct {
	name, folder string
	way          Way
}

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
	now := t0
	return launcherOver(t, fail, &now, func(s *core.Store) {
		for _, p := range projects {
			if err := s.AddProject(p); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// launcherOver opens the view over a temp store whose clock reads *now,
// once setup has filled it.
func launcherOver(t *testing.T, fail error, now *time.Time, setup func(*core.Store)) (fyne.Window, *[]opened, *bool) {
	t.Helper()
	s, err := core.Open(t.TempDir(), func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	setup(s)
	var got []opened
	w := test.NewWindow(nil)
	w.Resize(fyne.NewSize(720, 480))
	closed := false
	w.SetOnClosed(func() { closed = true })
	t.Cleanup(func() {
		if !closed { // the test driver panics on a second close
			w.Close()
		}
	})
	New(w, s, func(p core.Project, folder string, way Way) error {
		got = append(got, opened{p.Name, folder, way})
		return fail
	})
	return w, &got, &closed
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
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

	if len(*got) != 1 || (*got)[0] != (opened{"web", "/src/web", Way{Program: "code"}}) {
		t.Fatalf("opened %+v", *got)
	}
	if !*closed {
		t.Fatal("the window must close after a launch")
	}
}

func TestAProjectWithNoProgramsOpensATerminal(t *testing.T) {
	w, got, _ := launcher(t, core.Project{Name: "notes", Path: "/src/notes"})
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || (*got)[0] != (opened{"notes", "/src/notes", Way{}}) {
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

// slowStatuses makes the view's status source wait for release, the way a
// slow git repository would, and restores the real one after the test.
func slowStatuses(t *testing.T) (release func()) {
	t.Helper()
	gate := make(chan struct{})
	was := statusesOf
	statusesOf = func(s *core.Store) ([]core.Status, error) { <-gate; return s.Statuses() }
	t.Cleanup(func() { statusesOf = was })
	var once sync.Once
	return func() { once.Do(func() { close(gate) }) }
}

// label is a label on screen and where it is in the window.
type label struct {
	*widget.Label
	at fyne.Position
}

// labels is every visible label on screen. The test driver cannot place
// list rows, so positions are added up walking down from the content.
func labels(w fyne.Window) []label {
	var found []label
	var walk func(o fyne.CanvasObject, origin fyne.Position)
	walk = func(o fyne.CanvasObject, origin fyne.Position) {
		if !o.Visible() {
			return
		}
		at := origin.Add(o.Position())
		var kids []fyne.CanvasObject
		switch o := o.(type) {
		case *widget.Label:
			found = append(found, label{o, at})
			return
		case *fyne.Container:
			kids = o.Objects
		case fyne.Widget:
			kids = test.WidgetRenderer(o).Objects()
		}
		for _, k := range kids {
			walk(k, at)
		}
	}
	walk(w.Content(), fyne.Position{})
	return found
}

// texts is every label's text on screen, top to bottom, left pane before right.
func texts(w fyne.Window) []string {
	found := slices.DeleteFunc(labels(w), func(l label) bool { return l.Text == "" })
	split := w.Canvas().Size().Width * listShare // where the panes meet
	sort.SliceStable(found, func(i, j int) bool {
		if li, lj := found[i].at.X < split, found[j].at.X < split; li != lj {
			return li
		}
		return found[i].at.Y < found[j].at.Y
	})
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.Text
	}
	return out
}

// eventually waits for the screen to show want.
func eventually(t *testing.T, w fyne.Window, want string) []string {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if got := texts(w); slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, want) }) {
			return got
		}
	}
	t.Fatalf("never showed %q: %q", want, texts(w))
	return nil
}

func TestTheListShowsAtOnceAndSignalsFillInWhenStatusesReturn(t *testing.T) {
	release := slowStatuses(t)
	defer release()
	w, got, _ := launcher(t, core.Project{Name: "web", Path: t.TempDir(), Programs: []string{"code"}})

	if s := strings.Join(texts(w), "\n"); !strings.Contains(s, "web") || strings.Contains(s, "Quiet for") {
		t.Fatalf("before statuses: want the name and no note yet, got %q", s)
	}
	press(w, fyne.KeyReturn) // nothing waits on git
	if len(*got) != 1 {
		t.Fatalf("Enter before statuses opened %+v", *got)
	}
}

func TestTheRightPaneShowsNameStateAndNote(t *testing.T) {
	release := slowStatuses(t)
	w, _, _ := launcher(t, core.Project{Name: "web", Path: t.TempDir()})
	release()
	s := strings.Join(eventually(t, w, "Quiet for"), "\n")
	for _, want := range []string{"web", "ACTIVE", "Quiet for"} {
		if !strings.Contains(s, want) {
			t.Fatalf("want %q on screen: %q", want, s)
		}
	}
}

func TestTheListPutsQuietFirstThenSnoozedIdeasActiveAndArchivedAndMarksQuiet(t *testing.T) {
	const day = 24 * time.Hour
	now := t0
	w, _, _ := launcherOver(t, nil, &now, func(s *core.Store) {
		add := func(name, path string, at time.Time) int64 {
			now = at
			if err := s.AddProject(core.Project{Name: name, Path: path}); err != nil {
				t.Fatal(err)
			}
			ps, err := s.Projects()
			must(t, err)
			return ps[len(ps)-1].ID
		}
		archived := add("archived", t.TempDir(), t0)
		add("quiet-idea", "", t0.Add(15*day))
		add("active", t.TempDir(), t0.Add(29*day))
		add("newer-quiet", t.TempDir(), t0.Add(10*day))
		add("idea", "", t0.Add(29*day))
		snoozed := add("snoozed", t.TempDir(), t0)
		add("older-quiet", t.TempDir(), t0)
		now = t0.Add(30 * day)
		must(t, s.Drop(archived))
		must(t, s.Snooze(snoozed, 7*day))
	})
	got := eventually(t, w, "GONE QUIET")
	want := []string{"● older-quiet", "● newer-quiet", "● quiet-idea", "snoozed", "idea", "active", "archived"}
	if !slices.Equal(got[:len(want)], want) {
		t.Fatalf("list: want %q, got %q", want, got)
	}
}

func TestNoProjectsSaysSoOnceStatusesReturn(t *testing.T) {
	release := slowStatuses(t)
	w, _, _ := launcher(t)
	release()
	eventually(t, w, "No projects yet")
	time.Sleep(50 * time.Millisecond) // the fill-in must not crash on an empty list
}

func TestLongNamesAndNotesNeverWidenThePanesOrResizeTheWindow(t *testing.T) {
	long := strings.Repeat("very long project name ", 20)
	now := t0
	w, _, _ := launcherOver(t, nil, &now, func(s *core.Store) {
		must(t, s.AddProject(core.Project{Name: long, Path: t.TempDir()}))
		ps, err := s.Projects()
		must(t, err)
		must(t, s.AddMilestone(ps[0].ID, strings.Repeat("shipped the login page; next is signup ", 20)))
	})
	eventually(t, w, "Milestone: shipped")
	if size := w.Canvas().Size(); size != fyne.NewSize(720, 480) {
		t.Fatalf("the window resized to %v", size)
	}
	if min := w.Content().MinSize(); min.Width > 720 {
		t.Fatalf("content wants %v wide, wider than the window", min.Width)
	}
}

// tap clicks the label showing text, as a pointer would.
func tap(t *testing.T, w fyne.Window, text string) {
	t.Helper()
	for _, l := range labels(w) {
		if l.Text == text {
			test.TapCanvas(w.Canvas(), l.at.Add(fyne.NewPos(2, 2)))
			return
		}
	}
	t.Fatalf("no %q on screen: %q", text, texts(w))
}

func TestAProjectPickedWhileStatusesLoadStaysPickedAfterTheReorder(t *testing.T) {
	const day = 24 * time.Hour
	release := slowStatuses(t)
	now := t0
	w, got, _ := launcherOver(t, nil, &now, func(s *core.Store) {
		now = t0.Add(29 * day)
		must(t, s.AddProject(core.Project{Name: "fresh", Path: t.TempDir()}))
		now = t0.Add(5 * day)
		must(t, s.AddProject(core.Project{Name: "picked", Path: t.TempDir()}))
		now = t0
		must(t, s.AddProject(core.Project{Name: "quiet", Path: t.TempDir()})) // the top once sorted
		now = t0.Add(30 * day)
	})
	tap(t, w, "picked")
	release()
	eventually(t, w, "● quiet")
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || (*got)[0].name != "picked" {
		t.Fatalf("opened %+v", *got)
	}
}

// background is the window's background colour as the app's theme draws it now.
func background() color.Color {
	return fyne.CurrentApp().Settings().Theme().Color(theme.ColorNameBackground, fyne.CurrentApp().Settings().ThemeVariant())
}

func TestTheDarkToggleSwitchesTheLookAndRemembersIt(t *testing.T) {
	var s *core.Store
	now := t0
	w, _, _ := launcherOver(t, nil, &now, func(st *core.Store) { s = st; must(t, s.SetTheme(core.ThemeLight)) })
	t.Cleanup(func() { fyne.CurrentApp().Settings().SetTheme(nil) })
	light, dark := theme.DefaultTheme().Color(theme.ColorNameBackground, theme.VariantLight), theme.DefaultTheme().Color(theme.ColorNameBackground, theme.VariantDark)
	if background() != light {
		t.Fatalf("a light setting opens light, got %v", background())
	}
	tapCheck(t, w, "Dark")
	if got, err := s.Theme(); err != nil || got != core.ThemeDark || background() != dark {
		t.Fatalf("ticked: want dark saved and shown, got %q, %v, %v", got, err, background())
	}
	tapCheck(t, w, "Dark")
	if got, _ := s.Theme(); got != core.ThemeLight || background() != light {
		t.Fatalf("unticked: want light saved and shown, got %q, %v", got, background())
	}
}

// tapCheck taps the check box labelled text.
func tapCheck(t *testing.T, w fyne.Window, text string) {
	t.Helper()
	var find func(o fyne.CanvasObject) *widget.Check
	find = func(o fyne.CanvasObject) *widget.Check {
		switch o := o.(type) {
		case *widget.Check:
			if o.Text == text {
				return o
			}
		case *fyne.Container:
			for _, k := range o.Objects {
				if c := find(k); c != nil {
					return c
				}
			}
		case fyne.Widget:
			for _, k := range test.WidgetRenderer(o).Objects() {
				if c := find(k); c != nil {
					return c
				}
			}
		}
		return nil
	}
	c := find(w.Content())
	if c == nil {
		t.Fatalf("no %q check box", text)
	}
	test.Tap(c)
}

func TestEnterAfterStatusesReturnStillOpensTheMainProgram(t *testing.T) {
	release := slowStatuses(t)
	w, got, _ := launcher(t, core.Project{Name: "web", Path: t.TempDir(), Programs: []string{"code"}})
	release()
	eventually(t, w, "Quiet for")
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || (*got)[0].way != (Way{Program: "code"}) {
		t.Fatalf("opened %+v", *got)
	}
}

// typeText types s where the keyboard is, as a person would.
func typeText(w fyne.Window, s string) { test.Type(w.Canvas().Focused(), s) }

func TestTypingFiltersByNameAndEnterOpensTheMatch(t *testing.T) {
	slowStatuses(t) // never returns: the keys must not wait on it
	w, got, _ := launcher(t,
		core.Project{Name: "web", Path: "/src/web"},
		core.Project{Name: "Billing API", Path: "/src/api"},
		core.Project{Name: "webhooks", Path: "/src/hooks"})
	typeText(w, "api")
	if s := strings.Join(texts(w), "\n"); strings.Contains(s, "webhooks") || !strings.Contains(s, "Billing API") {
		t.Fatalf("filtered list: %q", s)
	}
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || (*got)[0].name != "Billing API" {
		t.Fatalf("opened %+v", *got)
	}
}

func TestUpAndDownMoveTheSelectionWhileTheCursorStaysInTheFilter(t *testing.T) {
	slowStatuses(t)
	w, got, _ := launcher(t,
		core.Project{Name: "web", Path: "/src/web"},
		core.Project{Name: "api", Path: "/src/api"},
		core.Project{Name: "docs", Path: "/src/docs"})
	filter := w.Canvas().Focused()
	for _, k := range []fyne.KeyName{fyne.KeyDown, fyne.KeyDown, fyne.KeyDown, fyne.KeyUp} { // the third Down stays on the last
		press(w, k)
	}
	if w.Canvas().Focused() != filter || filter == nil {
		t.Fatal("the cursor must stay in the filter box")
	}
	press(w, fyne.KeyReturn)
	if len(*got) != 1 || (*got)[0].name != "api" {
		t.Fatalf("opened %+v", *got)
	}
}

func TestEscClearsTheFilterThenASecondEscClosesTheWindow(t *testing.T) {
	slowStatuses(t)
	w, got, closed := launcher(t, core.Project{Name: "web", Path: "/src/web"})
	typeText(w, "zzz")
	press(w, fyne.KeyReturn)
	if len(*got) != 0 || !slices.Contains(texts(w), "No match.") {
		t.Fatalf("no match: opened %+v, screen %q", *got, texts(w))
	}
	press(w, fyne.KeyEscape)
	if *closed || !slices.Contains(texts(w), "web") {
		t.Fatalf("the first Esc clears the filter: closed %v, screen %q", *closed, texts(w))
	}
	press(w, fyne.KeyEscape)
	if !*closed || len(*got) != 0 {
		t.Fatalf("the second Esc closes without opening: closed %v, opened %+v", *closed, *got)
	}
}

func TestTheFilterBoxAndListNameThemselvesToScreenReaders(t *testing.T) {
	slowStatuses(t)
	w, _, _ := launcher(t, core.Project{Name: "web", Path: "/src/web"})
	var names []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if a, ok := o.(fyne.Accessible); ok {
			names = append(names, a.AccessibilityLabel())
		}
		switch o := o.(type) {
		case *fyne.Container:
			for _, k := range o.Objects {
				walk(k)
			}
		case fyne.Widget:
			for _, k := range test.WidgetRenderer(o).Objects() {
				walk(k)
			}
		}
	}
	walk(w.Content())
	for _, want := range []string{"Filter projects", "Projects"} {
		if !slices.Contains(names, want) {
			t.Fatalf("no %q for screen readers: %q", want, names)
		}
	}
}

func TestCtrlEditingKeysStillWorkInTheFilterBox(t *testing.T) {
	slowStatuses(t)
	w, _, _ := launcher(t, core.Project{Name: "web", Path: "/src/web"}, core.Project{Name: "api", Path: "/src/api"})
	typeText(w, "webzzz") // matches nothing until the word is deleted
	w.Canvas().Focused().(fyne.Shortcutable).TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyBackspace, Modifier: fyne.KeyModifierShortcutDefault})
	if s := texts(w); !slices.Contains(s, "web") || !slices.Contains(s, "api") {
		t.Fatalf("Ctrl+Backspace must delete the filter's last word: %q", s)
	}
}

// button finds the button labelled text on screen.
func button(t *testing.T, w fyne.Window, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch o := o.(type) {
		case *widget.Button:
			if o.Text == text && o.Visible() {
				found = o
			}
		case *fyne.Container:
			for _, k := range o.Objects {
				walk(k)
			}
		case fyne.Widget:
			for _, k := range test.WidgetRenderer(o).Objects() {
				walk(k)
			}
		}
	}
	walk(w.Content())
	if found == nil {
		t.Fatalf("no %q button: %q", text, texts(w))
	}
	return found
}

// moreWays is the "More ways to open" menu on screen.
func moreWays(t *testing.T, w fyne.Window) *widget.Select {
	t.Helper()
	var found *widget.Select
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch o := o.(type) {
		case *widget.Select:
			found = o
		case *fyne.Container:
			for _, k := range o.Objects {
				walk(k)
			}
		case fyne.Widget:
			for _, k := range test.WidgetRenderer(o).Objects() {
				walk(k)
			}
		}
	}
	walk(w.Content())
	if found == nil || found.PlaceHolder != "More ways to open" {
		t.Fatalf("no More ways to open menu")
	}
	return found
}

// agentsOnPath leaves only fake agent CLIs named names on PATH.
func agentsOnPath(t *testing.T, names ...string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake programs are shell scripts")
	}
	dir := t.TempDir()
	for _, n := range names {
		must(t, os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\n"), 0755))
	}
	t.Setenv("PATH", dir)
}

func TestTheMainButtonOpensTheFirstProgramOrATerminalAndCloses(t *testing.T) {
	slowStatuses(t)
	w, got, closed := launcher(t, core.Project{Name: "web", Path: "/src/web", Programs: []string{"/usr/bin/code", "zed"}})
	test.Tap(button(t, w, "Open in code"))
	if len(*got) != 1 || (*got)[0] != (opened{"web", "/src/web", Way{Program: "/usr/bin/code"}}) || !*closed {
		t.Fatalf("opened %+v, closed %v", *got, *closed)
	}

	w, got, _ = launcher(t, core.Project{Name: "notes", Path: "/src/notes"})
	test.Tap(button(t, w, "Terminal here"))
	if len(*got) != 1 || (*got)[0].way != (Way{}) {
		t.Fatalf("no programs: opened %+v", *got)
	}
}

func TestMoreWaysListsTheOtherProgramsTheTerminalAndAgentsOnPath(t *testing.T) {
	agentsOnPath(t, "aider", "claude")
	slowStatuses(t)
	w, got, closed := launcher(t, core.Project{Name: "web", Path: "/src/web", Programs: []string{"code", "zed"}})
	more := moreWays(t, w)
	want := []string{"Open in zed", "Terminal here", "Start Claude Code here", "Start aider here"}
	if !slices.Equal(more.Options, want) {
		t.Fatalf("options %q, want %q", more.Options, want)
	}
	more.SetSelected("Start aider here")
	if len(*got) != 1 || (*got)[0].way != (Way{Program: "aider", Agent: true}) || !*closed {
		t.Fatalf("opened %+v, closed %v", *got, *closed)
	}
}

func TestSnoozeAWeekAndDropActThroughTheStoreAndTheListUpdates(t *testing.T) {
	const day = 24 * time.Hour
	var s *core.Store
	now := t0
	w, _, _ := launcherOver(t, nil, &now, func(st *core.Store) {
		s = st
		must(t, s.AddProject(core.Project{Name: "web", Path: t.TempDir()}))
		must(t, s.AddProject(core.Project{Name: "api", Path: t.TempDir()}))
		now = t0.Add(30 * day)
	})
	eventually(t, w, "● web")
	ps, err := s.Projects()
	must(t, err)

	test.Tap(button(t, w, "Snooze a week"))
	eventually(t, w, "SNOOZED")
	if st, err := s.Status(ps[0].ID); err != nil || st.State != core.StateSnoozed || !st.SnoozedUntil.Equal(now.Add(7*day)) {
		t.Fatalf("web: %v until %v, %v", st.State, st.SnoozedUntil, err)
	}
	if got := texts(w); got[0] != "● api" || got[1] != "web" {
		t.Fatalf("the snoozed one moves below the quiet one: %q", got)
	}

	test.Tap(button(t, w, "Drop"))
	eventually(t, w, "ARCHIVED")
	if st, err := s.Status(ps[0].ID); err != nil || st.State != core.StateArchived {
		t.Fatalf("web: %v, %v", st.State, err)
	}
}
