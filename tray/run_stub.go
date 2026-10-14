//go:build !tray

package tray

import "errors"

// Available reports whether this binary was built with the tray.
const Available = false

// Run fails: this binary was built without the tray library.
func Run(Deps) error {
	return errors.New("this proz was built without the tray; install a release build, or build with: go build -tags tray")
}
