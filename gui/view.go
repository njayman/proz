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
// window, Esc clears the filter and then closes. The list shows at once;
// signals fill in as they come.
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
	picked := false // the user moved the selection
	agents := core.AgentsOnPath()
	var open func(Way)
	var act func(func(int64) error)

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
		if st.Path != "" { // ponytail: ideas get Set up… here with #93
			ways := waysToOpen(st.Project, agents)
			main := widget.NewButton(ways[0].label, func() { open(ways[0].Way) })
			main.Importance = widget.HighImportance
			labels := make([]string, len(ways)-1)
			for i, wy := range ways[1:] {
				labels[i] = wy.label
			}
			more := widget.NewSelect(labels, func(l string) {
				if i := slices.Index(labels, l); i >= 0 {
					open(ways[i+1].Way)
				}
			})
			more.PlaceHolder = "More ways to open"
			detail.Add(container.NewHBox(main, more))
		}
		if st.State != core.StateArchived {
			detail.Add(container.NewHBox(
				widget.NewButton("Snooze a week", func() { act(func(id int64) error { return s.Snooze(id, 7*24*time.Hour) }) }),
				widget.NewButton("Drop", func() { act(s.Drop) }),
			))
		}
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
	filter := newFilterBox(w.Canvas())
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
			if len(shown) > 0 {
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
	// ponytail: Ctrl+N and Ctrl+, are bound but do nothing until setup (#93) and settings (#92) exist
	for _, k := range []fyne.KeyName{fyne.KeyN, fyne.KeyComma} {
		w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) {})
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
	load := func() {
		go func() {
			sts, err := statusesOf(s)
			fyne.Do(func() {
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
	w.SetContent(container.NewBorder(container.NewHBox(layout.NewSpacer(), darkToggle(w, s)), nil, nil, nil, split))
	choose(0) // once the filter box is on the canvas, so it can take the focus
	w.Canvas().Focus(filter)
}

// filterBox is the filter entry. It hands the keys that move and open to
// onKey, so the cursor stays in it, and passes shortcuts to the window.
type filterBox struct {
	widget.Entry
	window fyne.Canvas
	onKey  func(fyne.KeyName)
}

func newFilterBox(window fyne.Canvas) *filterBox {
	f := &filterBox{window: window}
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
	if c, ok := s.(*desktop.CustomShortcut); ok && (c.KeyName == fyne.KeyN || c.KeyName == fyne.KeyComma) {
		if h, ok := f.window.(interface{ TypedShortcut(fyne.Shortcut) }); ok { // Ctrl+N and Ctrl+, belong to the window
			h.TypedShortcut(s)
		}
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

// way is a way to open a project, as the launcher names it.
type way struct {
	Way
	label string
}

// waysToOpen are the ways to open p, the main one first: its programs, then
// a terminal, then each agent. With no programs the terminal is the main one.
func waysToOpen(p core.Project, agents []core.Agent) []way {
	var out []way
	for _, prog := range p.Programs {
		out = append(out, way{Way{Program: prog}, "Open in " + filepath.Base(prog)})
	}
	out = append(out, way{Way{}, "Terminal here"})
	for _, a := range agents {
		out = append(out, way{Way{Program: a.Program, Agent: true}, "Start " + a.Name + " here"})
	}
	return out
}
