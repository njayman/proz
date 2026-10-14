package cmd

import (
	"fmt"
	"os"
	"os/exec"
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
		"'proz tray autostart on' starts it at login; 'proz config tray off'\n" +
		"stops a running tray and removes it from login.",
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
			Enabled: func() bool {
				on := true // if the setting cannot be read, keep running
				withStore(func(s *core.Store) (err error) { on, err = s.ModuleOn(core.ModuleTray); return err })
				return on
			},
			Check: func() error { _, err := runCheck(); return err },
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
	var cmd *exec.Cmd
	var p core.Project
	if err := withStore(func(s *core.Store) error {
		st, err := s.Status(id)
		if err != nil {
			return err
		}
		p = st.Project
		program := ""
		if len(p.Programs) > 0 {
			program = p.Programs[0]
		}
		cmd, err = s.Launch(p, program)
		return err
	}); err != nil {
		return err
	}
	if cmd == nil {
		return fmt.Errorf("%s has no folder or program to open; run 'proz resume %s' in a terminal", p.Name, p.Name)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap it when it exits, so no zombie lingers
	return nil
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
			if err := withStore(func(s *core.Store) error { return s.SetModule(core.ModuleTray, true) }); err != nil {
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
