//go:build !gui

package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

// The plain build leaves the GUI toolkit out entirely, so servers and the Pi
// keep a small cgo-free binary.
var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Open the launcher window (desktop builds only)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("this proz was built without the GUI; install a desktop release, or build with: go build -tags gui,accessibility")
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
