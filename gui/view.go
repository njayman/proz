// Package gui is the launcher: one window listing the projects, a place to
// open them. Only the entry point that opens a real window sits behind the
// gui build tag, so the view is tested headlessly in plain go test.
package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/njayman/proz/core"
)

// Launch opens project p in folder with program; an empty program means a
// terminal in that folder.
type Launch func(p core.Project, folder, program string) error

// New fills w with the launcher over s: the project list, where Enter opens
// the selected project with its main program and closes the window.
func New(w fyne.Window, s *core.Store, launch Launch) {
	projects, err := s.Projects()
	if err != nil {
		w.SetContent(widget.NewLabel("proz could not read your projects: " + err.Error()))
		return
	}
	selected := 0
	list := widget.NewList(
		func() int { return len(projects) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(projects[i].Name) },
	)
	list.OnSelected = func(i widget.ListItemID) { selected = i }
	if len(projects) > 0 {
		list.Select(0)
	}
	// ponytail: keys on the canvas; the filter box takes them over in #89
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name != fyne.KeyReturn && e.Name != fyne.KeyEnter || len(projects) == 0 {
			return
		}
		p := projects[selected]
		if err := launch(p, p.Path, mainProgram(p)); err != nil {
			dialog.ShowError(err, w)
			return
		}
		w.Close()
	})
	w.SetContent(list)
}

// mainProgram is the project's first program, or "" (the terminal) when it has none.
func mainProgram(p core.Project) string {
	if len(p.Programs) == 0 {
		return ""
	}
	return p.Programs[0]
}
