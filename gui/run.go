//go:build gui

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/njayman/proz/core"
)

// Run opens the launcher window over s and returns when it closes.
func Run(s *core.Store, launch Launch) error {
	a := app.NewWithID("io.github.njayman.proz")
	w := a.NewWindow("proz")
	New(w, s, launch)
	w.Resize(fyne.NewSize(720, 480)) // set once; views never resize it
	w.ShowAndRun()
	return nil
}
