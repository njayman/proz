package gui

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/njayman/proz/core"
)

// setup is the view that sets up p, or a new project or idea when p has
// no ID: its name and the programs it opens in. Save hands the edited
// project to save; Cancel, back or Esc leave it unsaved. It returns the view
// and the box the keys start in.
func setup(p core.Project, save func(core.Project) error, back func()) (fyne.CanvasObject, fyne.Focusable) {
	name := newEscEntry("Name", back)
	name.SetText(p.Name)
	// ponytail: programs are split on spaces, as proz edit's prompt does; a path with a space needs the CLI
	programs := newEscEntry("Such as code or nvim", back)
	programs.SetText(strings.Join(p.Programs, " "))
	problem := widget.NewLabel("")
	problem.Wrapping = fyne.TextWrapWord
	problem.Importance = widget.DangerImportance
	problem.Hide()
	do := func() {
		p.Name = strings.Join(strings.Fields(name.Text), " ")
		p.Programs = strings.Fields(programs.Text)
		err := errors.New("a project needs a name")
		if p.Name != "" {
			err = save(p)
		}
		if err != nil {
			problem.SetText(err.Error())
			problem.Show()
		}
	}
	name.OnSubmitted = func(string) { do() }
	programs.OnSubmitted = func(string) { do() }
	saveButton := widget.NewButton("Save", do)
	saveButton.Importance = widget.HighImportance

	title := "New project or idea"
	if p.ID != 0 {
		title = "Set up " + p.Name
	}
	// ponytail: folders (#94) and links (#95) join name and programs here
	return page(title, back, container.NewVBox(problem, container.NewHBox(saveButton, widget.NewButton("Cancel", back))),
		container.New(layout.NewFormLayout(),
			widget.NewLabel("Name"), name,
			widget.NewLabel("Programs"), programs,
		)), name
}
