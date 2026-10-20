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
// uiQueue holds what the view's background reads want to change on screen;
// settle applies it on the test's goroutine, so tests never race the view.
var uiQueue = make(chan func(), 100)

// settle applies every change queued so far.
func settle() {
	for {
		select {
		case f := <-uiQueue:
			f()
		default:
			return
		}
	}
}

func TestMain(m *testing.M) {
	onUI = func(f func()) { uiQueue <- f }
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
	settle()
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
	settle()
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
	test.Tap(find(t, w, func(c *widget.Check) bool { return c.Text == text }))
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
func typeText(w fyne.Window, s string) { settle(); test.Type(w.Canvas().Focused(), s) }

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
	for _, o := range objects(w) {
		if a, ok := o.(fyne.Accessible); ok {
			names = append(names, a.AccessibilityLabel())
		}
	}
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
	return find(t, w, func(b *widget.Button) bool { return b.Text == text && b.Visible() })
}

// moreWays is the "More ways to open" menu on screen.
func moreWays(t *testing.T, w fyne.Window) *widget.Select {
	t.Helper()
	return find(t, w, func(s *widget.Select) bool { return s.PlaceHolder == "More ways to open" })
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

// objects is everything the window draws, widgets and their parts.
func objects(w fyne.Window) []fyne.CanvasObject {
	settle()
	var out []fyne.CanvasObject
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		out = append(out, o)
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
	return out
}

// find is the first T on screen that match accepts.
func find[T fyne.CanvasObject](t *testing.T, w fyne.Window, match func(T) bool) T {
	t.Helper()
	for _, o := range objects(w) {
		if o, ok := o.(T); ok && match(o) {
			return o
		}
	}
	var none T
	t.Fatalf("no %T matching on screen: %q", none, texts(w))
	return none
}

// shortcut presses Ctrl and key, as the driver would: to the focused
// widget, else to the window.
func shortcut(w fyne.Window, key fyne.KeyName) {
	s := &desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierShortcutDefault}
	if f, ok := w.Canvas().Focused().(fyne.Shortcutable); ok {
		f.TypedShortcut(s)
	} else if h, ok := w.Canvas().(fyne.Shortcutable); ok {
		h.TypedShortcut(s)
	}
}

// showing says whether every one of want is on screen.
func showing(w fyne.Window, want ...string) bool {
	got := texts(w)
	for _, s := range want {
		if !slices.Contains(got, s) {
			return false
		}
	}
	return true
}

var settingsGroups = []string{"Reminders", "Features", "Email", "Launching"}

func TestCtrlCommaOrSettingsOpensSettingsInPlaceAndEscOrBackReturnsHome(t *testing.T) {
	slowStatuses(t)
	w, _, closed := launcher(t, core.Project{Name: "web", Path: "/src/web"})

	shortcut(w, fyne.KeyComma)
	if !showing(w, settingsGroups...) || showing(w, "web") {
		t.Fatalf("Ctrl+, shows settings in place of home: %q", texts(w))
	}
	press(w, fyne.KeyEscape)
	if !showing(w, "web") || showing(w, "Reminders") || *closed {
		t.Fatalf("Esc returns home: closed %v, %q", *closed, texts(w))
	}
	if _, ok := w.Canvas().Focused().(*filterBox); !ok {
		t.Fatal("home gives the keys back to the filter box")
	}

	test.Tap(button(t, w, "Settings"))
	if !showing(w, settingsGroups...) {
		t.Fatalf("Settings shows settings: %q", texts(w))
	}
	test.Tap(find(t, w, func(b *widget.Button) bool { return b.Icon == theme.NavigateBackIcon() }))
	if !showing(w, "web") || *closed {
		t.Fatalf("back returns home: %q", texts(w))
	}
	if size := w.Canvas().Size(); size != fyne.NewSize(720, 480) {
		t.Fatalf("the window resized to %v", size)
	}
}

func TestAnOlderStatusReadNeverOverwritesANewerOne(t *testing.T) {
	gate := make(chan struct{})
	var mu sync.Mutex
	first := true
	was := statusesOf
	statusesOf = func(s *core.Store) ([]core.Status, error) {
		mu.Lock()
		slow := first
		first = false
		mu.Unlock()
		sts, err := s.Statuses()
		if slow { // read before the snooze, landing after it
			<-gate
		}
		return sts, err
	}
	t.Cleanup(func() { statusesOf = was })
	now := t0
	w, _, _ := launcherOver(t, nil, &now, func(s *core.Store) {
		must(t, s.AddProject(core.Project{Name: "web", Path: t.TempDir()}))
		now = t0.Add(30 * 24 * time.Hour)
	})
	time.Sleep(50 * time.Millisecond) // the first read is in
	test.Tap(button(t, w, "Snooze a week"))
	eventually(t, w, "SNOOZED")
	close(gate)
	time.Sleep(100 * time.Millisecond)
	if says(w, "GONE QUIET") {
		t.Fatalf("the read from before the snooze won: %q", texts(w))
	}
}

// settingsOver opens the settings view over a temp store that setup fills,
// returning the window and the store.
func settingsOver(t *testing.T, setup func(*core.Store)) (fyne.Window, *core.Store) {
	t.Helper()
	slowStatuses(t)
	var s *core.Store
	now := t0
	w, _, _ := launcherOver(t, nil, &now, func(st *core.Store) { s = st; setup(s) })
	shortcut(w, fyne.KeyComma)
	return w, s
}

// entry is the text box on screen whose placeholder is hint.
func entry(t *testing.T, w fyne.Window, hint string) *escEntry {
	t.Helper()
	return find(t, w, func(e *escEntry) bool { return e.PlaceHolder == hint })
}

func TestQuietAfterAppliesAsItIsTyped(t *testing.T) {
	w, s := settingsOver(t, func(s *core.Store) { must(t, s.SetStaleThreshold(10*24*time.Hour)) })
	days := entry(t, w, "7")
	if days.Text != "10" {
		t.Fatalf("shows the saved days: %q", days.Text)
	}
	days.SetText("14")
	if d, err := s.StaleThreshold(); err != nil || d != 14*24*time.Hour {
		t.Fatalf("saved %v, %v", d, err)
	}
	days.SetText("0")
	if d, _ := s.StaleThreshold(); d != 14*24*time.Hour || !slices.ContainsFunc(texts(w), func(s string) bool { return strings.Contains(s, "whole days") }) {
		t.Fatalf("0 days is refused and says why: saved %v, screen %q", d, texts(w))
	}
}

// on reads module m from s.
func on(t *testing.T, s *core.Store, m core.Module) bool {
	t.Helper()
	v, err := s.ModuleOn(m)
	must(t, err)
	return v
}

// check is the check box on screen labelled text.
func check(t *testing.T, w fyne.Window, text string) *escCheck {
	t.Helper()
	return find(t, w, func(c *escCheck) bool { return c.Text == text })
}

func TestFeaturesSwitchModulesAtOnce(t *testing.T) {
	removed := 0
	was := removeAutostart
	removeAutostart = func() error { removed++; return nil }
	t.Cleanup(func() { removeAutostart = was })
	w, s := settingsOver(t, func(*core.Store) {})

	for _, c := range []struct {
		label string
		m     core.Module
	}{{"Notifications", core.ModuleNotifications}, {"Tray icon", core.ModuleTray}, {"Last agent prompt in notes", core.ModulePrompts}} {
		before := on(t, s, c.m)
		if check(t, w, c.label).Checked != before {
			t.Fatalf("%s shows %v, saved %v", c.label, !before, before)
		}
		test.Tap(check(t, w, c.label))
		if on(t, s, c.m) == before {
			t.Fatalf("%s: still %v after a tap", c.label, before)
		}
	}
	if removed != 0 {
		t.Fatalf("the tray switched on keeps its login item: removed %d times", removed)
	}
	test.Tap(check(t, w, "Tray icon")) // off again
	if removed != 1 || on(t, s, core.ModuleTray) {
		t.Fatalf("the tray switched off must not start at login either: removed %d times", removed)
	}

	for _, label := range []string{"Email digest", "Brain notes"} { // each needs setting up first
		test.Tap(check(t, w, label))
		if c := check(t, w, label); c.Checked || on(t, s, core.ModuleEmail) || on(t, s, core.ModuleBrain) {
			t.Fatalf("%s: shown %v with nothing set up", label, c.Checked)
		}
	}
	if !slices.ContainsFunc(texts(w), func(s string) bool { return strings.Contains(s, "brain folder") }) {
		t.Fatalf("says why: %q", texts(w))
	}
}

// weekly is the weekly email menu on screen.
func weekly(t *testing.T, w fyne.Window) *escSelect {
	t.Helper()
	return find(t, w, func(s *escSelect) bool { return s.Options[0] == "Off" })
}

var smtp = core.SMTP{Host: "smtp.example.com", Port: 587, From: "me@example.com", To: "me@example.com"}

func TestTheWeeklyEmailIsADayOrOff(t *testing.T) {
	w, s := settingsOver(t, func(s *core.Store) {
		must(t, s.SetSMTP(smtp))
		must(t, s.SetModule(core.ModuleEmail, true))
	})
	if got := weekly(t, w).Selected; got != "Monday" {
		t.Fatalf("shows the saved day: %q", got)
	}
	weekly(t, w).SetSelected("Friday")
	if d, err := s.DigestDay(); err != nil || d != time.Friday || !on(t, s, core.ModuleEmail) {
		t.Fatalf("Friday: saved %v, %v, email %v", d, err, on(t, s, core.ModuleEmail))
	}
	weekly(t, w).SetSelected("Off")
	if on(t, s, core.ModuleEmail) || check(t, w, "Email digest").Checked {
		t.Fatal("Off switches email off, and the feature shows it")
	}
	weekly(t, w).SetSelected("Tuesday")
	if d, _ := s.DigestDay(); d != time.Tuesday || !on(t, s, core.ModuleEmail) || !check(t, w, "Email digest").Checked {
		t.Fatalf("a day switches email back on: %v", d)
	}
	test.Tap(check(t, w, "Email digest"))
	if got := weekly(t, w).Selected; got != "Off" {
		t.Fatalf("the feature switched off shows Off: %q", got)
	}
}

func TestAWeeklyEmailDayWithNoMailServerSaysSoAndStaysOff(t *testing.T) {
	w, s := settingsOver(t, func(*core.Store) {})
	weekly(t, w).SetSelected("Friday")
	if d, _ := s.DigestDay(); d != time.Monday || on(t, s, core.ModuleEmail) || weekly(t, w).Selected != "Off" {
		t.Fatalf("saved %v, email %v, shown %q", d, on(t, s, core.ModuleEmail), weekly(t, w).Selected)
	}
	if !says(w, "fill in the mail server") {
		t.Fatalf("says why: %q", texts(w))
	}
}

func TestTheMailServerSavesOnceItIsComplete(t *testing.T) {
	w, s := settingsOver(t, func(*core.Store) {})
	if got := entry(t, w, "587").Text; got != "587" {
		t.Fatalf("a new mail server starts on port 587: %q", got)
	}
	test.Type(entry(t, w, "smtp.example.com"), "smtp.example.com")
	if _, ok, _ := s.SMTP(); ok || !slices.ContainsFunc(texts(w), func(s string) bool { return strings.Contains(s, "mail server not saved") }) {
		t.Fatalf("saved without addresses, or did not say: %v, %q", ok, texts(w))
	}
	test.Type(entry(t, w, "Sender address"), "me@example.com")
	test.Type(entry(t, w, "Your address"), "me@example.com")
	test.Type(entry(t, w, "Password"), "s3cret")
	got, ok, err := s.SMTP()
	if want := (core.SMTP{Host: "smtp.example.com", Port: 587, Password: "s3cret", From: "me@example.com", To: "me@example.com"}); err != nil || !ok || got != want {
		t.Fatalf("saved %+v, %v, %v", got, ok, err)
	}
	if slices.ContainsFunc(texts(w), func(s string) bool { return strings.Contains(s, "not saved") }) {
		t.Fatalf("the problem clears once saved: %q", texts(w))
	}
	entry(t, w, "587").SetText("465")
	if got, _, _ := s.SMTP(); got.Port != 465 {
		t.Fatalf("port: %d", got.Port)
	}
}

func TestASavedMailServerShowsAsSaved(t *testing.T) {
	w, _ := settingsOver(t, func(s *core.Store) { must(t, s.SetSMTP(smtp)) })
	if got := entry(t, w, "smtp.example.com").Text; got != smtp.Host {
		t.Fatalf("host: %q", got)
	}
	if got := entry(t, w, "Your address").Text; got != smtp.To {
		t.Fatalf("to: %q", got)
	}
}

// says reports whether some text on screen contains want.
func says(w fyne.Window, want string) bool {
	return slices.ContainsFunc(texts(w), func(s string) bool { return strings.Contains(s, want) })
}

func TestTheFirstMailServerSwitchesEmailOnAsTheCLIDoes(t *testing.T) {
	w, s := settingsOver(t, func(*core.Store) {})
	test.Type(entry(t, w, "smtp.example.com"), "smtp.example.com")
	test.Type(entry(t, w, "Sender address"), "me@example.com")
	test.Type(entry(t, w, "Your address"), "me@example.com")
	if !on(t, s, core.ModuleEmail) || !check(t, w, "Email digest").Checked || weekly(t, w).Selected != "Monday" {
		t.Fatalf("email %v, shown %v, %q", on(t, s, core.ModuleEmail), check(t, w, "Email digest").Checked, weekly(t, w).Selected)
	}
}

func TestASavedPasswordNeverGoesToANewServer(t *testing.T) {
	saved := smtp
	saved.Password = "s3cret"
	w, s := settingsOver(t, func(s *core.Store) { must(t, s.SetSMTP(saved)) })
	if pw := entry(t, w, "Saved; type to change").Text; pw != "" {
		t.Fatalf("the password box shows the saved password: %q", pw)
	}
	entry(t, w, "smtp.example.com").SetText("evil.example.com")
	if got, _, _ := s.SMTP(); got.Host != smtp.Host || !says(w, "password again") {
		t.Fatalf("a new host with the old password: saved %q, screen %q", got.Host, texts(w))
	}
	test.Type(entry(t, w, "Saved; type to change"), "other")
	if got, _, _ := s.SMTP(); got.Host != "evil.example.com" || got.Password != "other" {
		t.Fatalf("a new host with its password typed: %+v", got)
	}
}

func TestAProblemStaysShownUntilItsOwnSettingIsFixed(t *testing.T) {
	w, _ := settingsOver(t, func(*core.Store) {})
	test.Type(entry(t, w, "smtp.example.com"), "smtp.example.com")
	test.Tap(check(t, w, "Notifications"))
	if !says(w, "mail server not saved") {
		t.Fatalf("another setting saving hides the mail server's problem: %q", texts(w))
	}
}

func TestEmailWithNoMailServerPointsAtTheFieldsNotTheCLI(t *testing.T) {
	w, _ := settingsOver(t, func(*core.Store) {})
	test.Tap(check(t, w, "Email digest"))
	if !says(w, "fill in the mail server") || says(w, "proz config") {
		t.Fatalf("%q", texts(w))
	}
}

func TestTheTerminalOverrideAppliesAsItIsTypedAndEmptyDetects(t *testing.T) {
	w, s := settingsOver(t, func(*core.Store) {})
	test.Type(entry(t, w, "Detect"), "wezterm")
	if got, err := s.Terminal(); err != nil || got != "wezterm" {
		t.Fatalf("saved %q, %v", got, err)
	}
	entry(t, w, "Detect").SetText("")
	if got, _ := s.Terminal(); got != "" {
		t.Fatalf("empty detects again: %q", got)
	}
}

func TestEscFromATextBoxCheckBoxOrMenuInSettingsReturnsHome(t *testing.T) {
	w, _ := settingsOver(t, func(s *core.Store) { must(t, s.AddProject(core.Project{Name: "web", Path: "/src/web"})) })
	w.Canvas().Focus(entry(t, w, "Detect"))
	press(w, fyne.KeyEscape)
	if !showing(w, "web") {
		t.Fatalf("Esc in a text box: %q", texts(w))
	}
	shortcut(w, fyne.KeyComma)
	test.Tap(check(t, w, "Notifications"))
	press(w, fyne.KeyEscape)
	if !showing(w, "web") {
		t.Fatalf("Esc after a tap on a check box: %q", texts(w))
	}
	shortcut(w, fyne.KeyComma)
	w.Canvas().Focus(weekly(t, w))
	press(w, fyne.KeyEscape)
	if !showing(w, "web") {
		t.Fatalf("Esc on the weekly email menu: %q", texts(w))
	}
}
