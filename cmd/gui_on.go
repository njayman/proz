//go:build gui

package cmd

import (
	"os/exec"

	"github.com/njayman/proz/core"
	"github.com/njayman/proz/gui"
	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Open the launcher window",
	Long: "Open a window listing your projects. Enter opens the selected one\n" +
		"with its first program, or a terminal in its folder when it has none,\n" +
		"and the window closes.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error { return gui.Run(s, guiLaunch(s)) })
	},
}

// guiLaunch opens projects from the launcher, where there is no terminal:
// the program starts on its own, terminal programs in the user's terminal.
func guiLaunch(s *core.Store) gui.Launch {
	return func(p core.Project, folder, program string) error {
		p.Path = folder
		var cmd *exec.Cmd
		var err error
		if program == "" {
			cmd, err = s.LaunchTerminal(p, false)
		} else {
			cmd, err = s.Launch(p, program, false)
		}
		if err != nil || cmd == nil {
			return err
		}
		return startDetached(cmd)
	}
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
