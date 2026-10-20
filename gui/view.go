// Package gui is the launcher: one window listing the projects, a place to
// open them. Only the entry point that opens a real window sits behind the
// gui build tag, so the view is tested headlessly in plain go test.
package gui

import (
	"cmp"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/njayman/proz/core"
)

// Launch opens project p in folder with program; an empty program means a
// terminal in that folder.
type Launch func(p core.Project, folder, program string) error

// statusesOf reads every project's signals; tests swap in a slow one.
var statusesOf = (*core.Store).Statuses

// New fills w with the launcher over s: the project list on the left, the
// selected project on the right, where Enter opens it with its main program
// and closes the window. The list shows at once; signals fill in as they come.
func New(w fyne.Window, s *core.Store, launch Launch) {
	projects, err := s.Projects()
	if err != nil {
		w.SetContent(widget.NewLabel("proz could not read your projects: " + err.Error()))
		return
	}
	shown := make([]core.Status, len(projects)) // no State until statuses return
	for i, p := range projects {
		shown[i] = core.Status{Project: p}
	}
	selected := 0

	list := widget.NewList(
		func() int { return len(shown) },
		func() fyne.CanvasObject { return truncated("", fyne.TextStyle{}) },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			mark := ""
			if shown[i].State == core.StateStale {
				mark = "● "
			}
			o.(*widget.Label).SetText(mark + shown[i].Name)
		},
	)
	list.HideSeparators = true
	detail := container.NewVBox()
	render := func() {
		detail.RemoveAll()
		if len(shown) == 0 {
			detail.Add(widget.NewLabel("No projects yet."))
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
	}
	picked := false // the user moved the selection
	if len(shown) > 0 {
		list.Select(0)
	}
	render()
	list.OnSelected = func(i widget.ListItemID) {
		selected, picked = i, true
		render()
		w.Canvas().Unfocus() // ponytail: keeps Enter on the canvas until #89's filter box owns the keys
	}
	go func() {
		sts, err := statusesOf(s)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			var was int64
			if len(shown) > 0 {
				was = shown[selected].ID
			}
			shown = remindOrder(sts)
			list.Refresh()
			// a choice the user made survives the reorder; else the top one
			selected = 0
			if i := slices.IndexFunc(shown, func(st core.Status) bool { return st.ID == was }); picked && i >= 0 {
				selected = i
			}
			if len(shown) > 0 {
				list.Select(selected)
			}
			render()
		})
	}()

	// ponytail: keys on the canvas; the filter box takes them over in #89
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name != fyne.KeyReturn && e.Name != fyne.KeyEnter || len(shown) == 0 {
			return
		}
		p := shown[selected].Project
		if err := launch(p, p.Path, mainProgram(p)); err != nil {
			dialog.ShowError(err, w)
			return
		}
		w.Close()
	})
	split := container.NewHSplit(list, container.NewVScroll(container.NewPadded(detail)))
	split.Offset = 0.3
	w.SetContent(split)
}

// remindOrder puts quiet projects first, then snoozed ones, ideas, active
// and archived; within a group, the longest quiet first.
func remindOrder(sts []core.Status) []core.Status {
	group := func(st core.Status) int {
		switch {
		case st.State == core.StateArchived:
			return 4
		case st.Path == "":
			return 2
		case st.State == core.StateStale:
			return 0
		case st.State == core.StateSnoozed:
			return 1
		}
		return 3
	}
	slices.SortStableFunc(sts, func(a, b core.Status) int {
		return cmp.Or(cmp.Compare(group(a), group(b)), cmp.Compare(b.Quiet, a.Quiet))
	})
	return sts
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
	switch {
	case st.State == "":
		return ""
	case st.State == core.StateArchived:
		return "archived"
	case st.Path == "":
		return "idea"
	case st.State == core.StateStale:
		return "gone quiet"
	}
	return string(st.State)
}

// mainProgram is the project's first program, or "" (the terminal) when it has none.
func mainProgram(p core.Project) string {
	if len(p.Programs) == 0 {
		return ""
	}
	return p.Programs[0]
}
