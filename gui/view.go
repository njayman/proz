// Package gui is the launcher: one window listing the projects, a place to
// open them. Only the entry point that opens a real window sits behind the
// gui build tag, so the view is tested headlessly in plain go test.
package gui

import (
	"image/color"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/njayman/proz/core"
)

// Way is how to open a project: with a program, an agent CLI (Agent set)
// or, the zero Way, a terminal.
type Way struct {
	Program string
	Agent   bool
}

// Launch opens project p in folder the way given.
type Launch func(p core.Project, folder string, way Way) error

// listShare is the part of the window's width the list takes.
const listShare = 0.3

// statusesOf reads every project's signals; tests swap in a slow one.
var statusesOf = (*core.Store).Statuses

// New fills w with the launcher over s: a filter box over the project list
// on the left, the selected project on the right. Typing filters, Up and Down
// move, Enter opens the selection with its main program and closes the
// window (or sets up an idea), Esc clears the filter and then closes. Ctrl+N
// shows setup for a new project in its place, Ctrl+, the settings. The list
// shows at once; signals fill in as they come.
func New(w fyne.Window, s *core.Store, launch Launch) {
	projects, err := s.Projects()
	if err != nil {
		w.SetContent(widget.NewLabel("proz could not read your projects: " + err.Error()))
		return
	}
	all := make([]core.Status, len(projects)) // no State until statuses return
	for i, p := range projects {
		all[i] = core.Status{Project: p}
	}
	shown, selected := all, 0
	picked := false                  // the user moved the selection
	agents := core.AgentCLIsOnPath() // ponytail: read once per window; the tray-started window has the session PATH
	var open func(Way)
	var act func(func(int64) error)
	var showSetup func(core.Project)

	list := newProjectList(
		func() int { return len(shown) },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			mark := ""
			if shown[i].State == core.StateStale {
				mark = "● "
			}
			o.(*widget.Label).SetText(mark + shown[i].Name)
		},
	)
	detail := container.NewVBox()
	render := func() {
		detail.RemoveAll()
		switch {
		case len(all) == 0:
			detail.Add(widget.NewLabel("No projects yet."))
			return
		case len(shown) == 0:
			detail.Add(widget.NewLabel("No match."))
			return
		}
		st := shown[selected]
		detail.Add(truncated(st.Name, fyne.TextStyle{Bold: true}))
		detail.Add(truncated(strings.ToUpper(stateWord(st)), fyne.TextStyle{}))
		detail.Add(widget.NewSeparator())
		detail.Add(widget.NewLabelWithStyle("Where you left off", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
		for _, l := range st.Note.Lines {
			note := widget.NewLabel(l)
			note.Wrapping = fyne.TextWrapWord
			detail.Add(note)
		}
		detail.Add(widget.NewSeparator())
		if st.Path != "" { // an idea has no folder to open yet
			ways := waysToOpen(st.Project, agents)
			main := widget.NewButton(ways[0].label, func() { open(ways[0].Way) })
			main.Importance = widget.HighImportance
			labels := make([]string, len(ways)-1)
			for i, wy := range ways[1:] {
				labels[i] = wy.label
			}
			var more *widget.Select
			more = widget.NewSelect(labels, func(l string) {
				if i := slices.Index(labels, l); i >= 0 {
					open(ways[i+1].Way)
					more.ClearSelected() // still open after a failed launch: back to the placeholder
				}
			})
			more.PlaceHolder = "More ways to open"
			detail.Add(container.NewHBox(main, more))
		}
		row := container.NewHBox(widget.NewButton("Set up…", func() { showSetup(st.Project) }))
		if st.State != core.StateArchived {
			row.Add(widget.NewButton("Snooze a week", func() { act(func(id int64) error { return s.Snooze(id, 7*24*time.Hour) }) }))
			row.Add(widget.NewButton("Drop", func() { act(s.Drop) }))
		}
		detail.Add(row)
	}
	// choose selects row i of what is shown, or nothing when it is empty.
	choose := func(i int) {
		selected = max(0, min(i, len(shown)-1))
		if len(shown) > 0 {
			list.Select(selected)
			list.ScrollTo(selected)
		} else {
			list.UnselectAll()
		}
		render()
	}
	ctrl := map[fyne.KeyName]func(){} // the window's Ctrl keys, filled in below
	filter := newFilterBox(ctrl)
	// refilter shows the projects matching the filter, keeping the selected
	// one selected when it still matches; else the top one.
	refilter := func() {
		var was int64
		if len(shown) > 0 {
			was = shown[selected].ID
		}
		q := strings.ToLower(filter.Text)
		shown = slices.DeleteFunc(slices.Clone(all), func(st core.Status) bool { return !strings.Contains(strings.ToLower(st.Name), q) })
		list.Refresh()
		i := slices.IndexFunc(shown, func(st core.Status) bool { return st.ID == was })
		if !picked || i < 0 {
			i = 0
		}
		choose(i)
	}
	filter.OnChanged = func(string) { picked = false; refilter() }
	list.OnSelected = func(i widget.ListItemID) {
		if i != selected {
			selected, picked = i, true
			render()
		}
		w.Canvas().Focus(filter) // a click never takes the keys from the filter box
	}
	filter.onKey = func(k fyne.KeyName) {
		switch k {
		case fyne.KeyDown, fyne.KeyUp:
			step := 1
			if k == fyne.KeyUp {
				step = -1
			}
			picked = true
			choose(selected + step)
		case fyne.KeyReturn, fyne.KeyEnter:
			switch {
			case len(shown) == 0:
			case shown[selected].Path == "": // an idea opens its setup
				showSetup(shown[selected].Project)
			default:
				open(waysToOpen(shown[selected].Project, nil)[0].Way)
			}
		case fyne.KeyEscape:
			if filter.Text != "" {
				filter.SetText("")
				return
			}
			w.Close()
		}
	}
	var home fyne.CanvasObject
	var load func()
	goHome := func() {
		w.Canvas().SetOnTypedKey(nil)
		w.SetContent(home)
		w.Canvas().Focus(filter)
		load() // a setting may have changed what is quiet
	}
	// show replaces home with view, which leaves by goHome; so does Esc.
	show := func(view fyne.CanvasObject) {
		w.SetContent(view)
		w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
			if e.Name == fyne.KeyEscape {
				goHome()
			}
		})
	}
	showSettings := func() { show(settings(s, goHome)) }
	// saved writes p through the store and goes home with it selected,
	// keeping the signals already in until load brings fresh ones.
	saved := func(p core.Project) error {
		add := p.ID == 0
		var err error
		if add {
			err = s.AddProject(p)
		} else {
			err = s.UpdateProject(p)
		}
		if err != nil {
			return err
		}
		ps, err := s.Projects()
		if err != nil {
			return err
		}
		if add {
			p.ID = ps[len(ps)-1].ID // ponytail: the newest is ours; an agent adding one in the same instant could swap them
		}
		had := map[int64]core.Status{}
		for _, st := range all {
			had[st.ID] = st
		}
		projects, all = ps, make([]core.Status, len(ps))
		for i, q := range ps {
			all[i] = had[q.ID]
			all[i].Project = q
		}
		core.RemindOrder(all)
		filter.SetText("") // so it is listed
		shown, picked = all, true
		list.Refresh()
		choose(slices.IndexFunc(shown, func(st core.Status) bool { return st.ID == p.ID }))
		goHome()
		return nil
	}
	showSetup = func(p core.Project) {
		view, first := setup(p, saved, goHome)
		show(view)
		w.Canvas().Focus(first)
	}
	newProject := func() { showSetup(core.Project{}) }
	ctrl[fyne.KeyComma] = showSettings
	ctrl[fyne.KeyN] = newProject
	for k, do := range ctrl { // for views without the filter box
		w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) { do() })
	}

	// open launches the selected project the way given and closes the window.
	open = func(way Way) {
		p := shown[selected].Project
		if err := launch(p, p.Path, way); err != nil {
			dialog.ShowError(err, w)
			return
		}
		w.Close()
	}
	loads := 0 // reads started, so an older one landing late is dropped
	load = func() {
		loads++
		this := loads
		go func() {
			sts, err := statusesOf(s)
			fyne.Do(func() {
				if this != loads {
					return
				}
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				byID := map[int64]core.Project{}
				for _, p := range projects {
					byID[p.ID] = p
				}
				for i := range sts { // statuses carry signals, not programs
					if p, ok := byID[sts[i].ID]; ok {
						sts[i].Project = p
					}
				}
				core.RemindOrder(sts)
				all = sts
				refilter() // a choice the user made survives the reorder
			})
		}()
	}
	// act applies a response to the selected project's reminder, then reads
	// the statuses again, keeping that project selected.
	act = func(respond func(int64) error) {
		if err := respond(shown[selected].ID); err != nil {
			dialog.ShowError(err, w)
			return
		}
		picked = true
		load()
	}
	load()

	split := container.NewHSplit(container.NewBorder(filter, nil, nil, nil, list), container.NewVScroll(container.NewPadded(detail)))
	split.Offset = listShare
	toolbar := container.NewHBox(widget.NewButtonWithIcon("New…", theme.ContentAddIcon(), newProject), layout.NewSpacer(),
		darkToggle(w, s), widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), showSettings))
	home = container.NewBorder(toolbar, nil, nil, nil, split)
	w.SetContent(home)
	choose(0) // once the filter box is on the canvas, so it can take the focus
	w.Canvas().Focus(filter)
}

// filterBox is the filter entry. It hands the keys that move and open to
// onKey, so the cursor stays in it, and the window's Ctrl keys to ctrl.
type filterBox struct {
	widget.Entry
	onKey func(fyne.KeyName)
	ctrl  map[fyne.KeyName]func()
}

func newFilterBox(ctrl map[fyne.KeyName]func()) *filterBox {
	f := &filterBox{ctrl: ctrl}
	f.ExtendBaseWidget(f)
	f.PlaceHolder = "Filter"
	return f
}

func (f *filterBox) TypedKey(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyDown, fyne.KeyUp, fyne.KeyReturn, fyne.KeyEnter, fyne.KeyEscape:
		f.onKey(e.Name)
	default:
		f.Entry.TypedKey(e)
	}
}

func (f *filterBox) TypedShortcut(s fyne.Shortcut) {
	if c, ok := s.(*desktop.CustomShortcut); ok && c.Modifier == fyne.KeyModifierShortcutDefault && f.ctrl[c.KeyName] != nil {
		f.ctrl[c.KeyName]()
		return
	}
	f.Entry.TypedShortcut(s) // its own word editing, such as Ctrl+Backspace
}

func (f *filterBox) AccessibilityLabel() string             { return "Filter projects" }
func (f *filterBox) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleText }

// projectList is the list of projects, named for screen readers.
type projectList struct{ widget.List }

func newProjectList(length func() int, update func(widget.ListItemID, fyne.CanvasObject)) *projectList {
	l := &projectList{}
	l.Length = length
	l.CreateItem = func() fyne.CanvasObject { return truncated("", fyne.TextStyle{}) }
	l.UpdateItem = update
	l.HideSeparators = true
	l.ExtendBaseWidget(l)
	return l
}

func (l *projectList) AccessibilityLabel() string             { return "Projects" }
func (l *projectList) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleContainer }

// darkToggle applies the saved look and is the check box that switches it:
// ticked is dark, unticked light. Until it is used, the system decides.
func darkToggle(w fyne.Window, s *core.Store) *widget.Check {
	settings := fyne.CurrentApp().Settings()
	look, err := s.Theme()
	if err != nil {
		dialog.ShowError(err, w)
	}
	apply := func(t core.Theme) {
		if t == core.ThemeSystem {
			settings.SetTheme(nil) // ponytail: nil restores Fyne's default, which follows the system
			return
		}
		v := theme.VariantLight
		if t == core.ThemeDark {
			v = theme.VariantDark
		}
		settings.SetTheme(fixedVariant{theme.DefaultTheme(), v})
	}
	apply(look)
	dark := widget.NewCheck("Dark", func(on bool) {
		t := core.ThemeLight
		if on {
			t = core.ThemeDark
		}
		if err := s.SetTheme(t); err != nil {
			dialog.ShowError(err, w)
			return
		}
		apply(t)
	})
	dark.Checked = look == core.ThemeDark || look == core.ThemeSystem && settings.ThemeVariant() == theme.VariantDark
	return dark
}

// fixedVariant is a theme that draws one variant whatever the system says.
type fixedVariant struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (f fixedVariant) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return f.Theme.Color(n, f.variant)
}

// truncated is a one-line label that ends in an ellipsis rather than
// widening its pane.
func truncated(text string, style fyne.TextStyle) *widget.Label {
	l := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, style)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// stateWord is how the right pane names a project's state; empty until
// its status is in.
func stateWord(st core.Status) string {
	word := string(st.State)
	if st.State == core.StateStale {
		word = "gone quiet"
	}
	if st.Path == "" && st.State != "" { // ideas say so whatever their state
		if st.State == core.StateActive {
			return "idea"
		}
		return "idea, " + word
	}
	return word
}

// wayOption is a way to open a project, as the launcher names it.
type wayOption struct {
	Way
	label string
}

// waysToOpen are the ways to open p, the main one first: its programs, then
// a terminal, then each agent. With no programs the terminal is the main one.
func waysToOpen(p core.Project, agents []core.AgentCLI) []wayOption {
	var out []wayOption
	for _, prog := range p.Programs {
		out = append(out, wayOption{Way{Program: prog}, "Open in " + filepath.Base(prog)})
	}
	out = append(out, wayOption{Way{}, "Terminal here"})
	for _, a := range agents {
		out = append(out, wayOption{Way{Program: a.Program, Agent: true}, "Start " + a.Name + " here"})
	}
	return out
}
