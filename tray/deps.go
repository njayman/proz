package tray

import (
	"time"

	"github.com/njayman/proz/core"
)

// Deps is what the tray needs from the rest of proz. The CLI supplies it, so
// the tray holds no domain rules.
type Deps struct {
	Interval time.Duration
	Enabled  func() bool                   // false once the user switches the tray off; it then quits
	Check    func() error                  // one iteration: notify what just went quiet
	Nudges   func() ([]core.Status, error) // what the menu lists
	Resume   func(id int64) error
	Snooze   func(id int64) error
	Drop     func(id int64) error
	Log      func(error)
}
