package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/njayman/proz/autostart"
	"github.com/njayman/proz/core"
	"github.com/njayman/proz/tray"
	"github.com/spf13/cobra"
)

// trayInterval is how often the tray checks. ponytail: fixed; make it a setting if anyone needs faster.
const trayInterval = 15 * time.Minute

var trayCmd = &cobra.Command{
	Use:   "tray",
	Short: "Show a tray icon with a dot when a project goes quiet",
	Long: "Run in the background with a tray icon. It checks every 15 minutes,\n" +
		"shows a dot while any project is quiet, and notifies once per quiet\n" +
		"stretch. The menu resumes, snoozes or drops each quiet project.\n" +
		"'proz tray autostart on' starts it at login.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !tray.Available {
			return tray.Run(tray.Deps{})
		}
		if err := withStore(func(s *core.Store) error { return s.SetModule(core.ModuleTray, true) }); err != nil {
			return err
		}
		return tray.Run(tray.Deps{
			Interval: trayInterval,
			Check:    func() error { _, err := runCheck(); return err },
			Nudges: func() (ns []core.Status, err error) {
				err = withStore(func(s *core.Store) error { ns, err = s.Nudges(); return err })
				return ns, err
			},
			Resume: resumeDetached,
			Snooze: func(id int64) error {
				return withStore(func(s *core.Store) error { return s.Snooze(id, 7*24*time.Hour) })
			},
			Drop: func(id int64) error { return withStore(func(s *core.Store) error { return s.Drop(id) }) },
			Log:  func(err error) { fmt.Fprintf(os.Stderr, "proz tray: %v\n", err) },
		})
	},
}

// resumeDetached opens a project from the tray, where there is no terminal:
// the program starts on its own. ponytail: first program only; terminal
// editors (vim) need a terminal and will not show; use 'proz resume' there.
func resumeDetached(id int64) error {
	var cmd interface{ Start() error }
	err := withStore(func(s *core.Store) error {
		projects, err := s.Projects()
		if err != nil {
			return err
		}
		for _, p := range projects {
			if p.ID != id {
				continue
			}
			program := ""
			if len(p.Programs) > 0 {
				program = p.Programs[0]
			}
			c, err := s.Launch(p, program)
			if c != nil {
				cmd = c
			}
			return err
		}
		return core.ErrNotFound
	})
	if err != nil || cmd == nil {
		return err
	}
	return cmd.Start()
}

var trayAutostartCmd = &cobra.Command{
	Use:       "autostart [on|off]",
	Short:     "Show, or switch on or off, starting the tray at login",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"on", "off"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			if autostart.Installed() {
				fmt.Println("on")
			} else {
				fmt.Println("off")
			}
			return nil
		}
		switch args[0] {
		case "on":
			if !tray.Available {
				return tray.Run(tray.Deps{}) // explains how to get a tray build
			}
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			if err := autostart.Install(exe); err != nil {
				return err
			}
			fmt.Println("The tray will start at login.")
		case "off":
			if err := autostart.Remove(); err != nil {
				return err
			}
			fmt.Println("The tray will no longer start at login.")
		default:
			return fmt.Errorf("want on or off, got %q", args[0])
		}
		return nil
	},
}

func init() {
	trayCmd.AddCommand(trayAutostartCmd)
	rootCmd.AddCommand(trayCmd)
}
