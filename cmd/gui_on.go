//go:build gui

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/njayman/proz/core"
	"github.com/njayman/proz/gui"
	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Open the launcher window",
	Long: "Open a window listing your projects. Type to filter, Up and Down to\n" +
		"move. Enter opens the selected one with its first program, or a\n" +
		"terminal in its folder when it has none, and the window closes; More\n" +
		"ways to open offers its other programs, a terminal and agent CLIs.\n" +
		"Esc clears the filter, then closes the window.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error { return gui.Run(s, guiLaunch(s)) })
	},
}

// guiLaunch opens projects from the launcher, where there is no terminal:
// the program starts on its own, terminal programs in the user's terminal.
func guiLaunch(s *core.Store) gui.Launch {
	return func(p core.Project, folder string, way gui.Way) error {
		p.Path = folder
		var cmd *exec.Cmd
		var err error
		switch {
		case way.Agent:
			cmd, err = s.LaunchAgent(p, way.Program, false)
		case way.Program == "":
			cmd, err = s.LaunchTerminal(p, false)
		default:
			cmd, err = s.Launch(p, way.Program, false)
		}
		if err != nil {
			return err
		}
		if cmd == nil { // ponytail: ideas open setup once #93 lands
			return fmt.Errorf("%s is an idea with no folder yet; run 'proz add' in its folder", p.Name)
		}
		return startDetached(cmd)
	}
}

// openLauncher starts proz gui as its own process, so the tray never hosts
// the window and stays free of the GUI toolkit's main loop.
func openLauncher() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return startDetached(exec.Command(exe, "gui"))
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
