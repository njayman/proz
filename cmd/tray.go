package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/njayman/proz/autostart"
	"github.com/njayman/proz/core"
	"github.com/njayman/proz/notify"
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
		"stops a running tray within 30 seconds and removes it from login.",
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
			Enabled:  trayEnabled,
			Check:    func() error { _, err := runCheck(); return err },
			Nudges: func() (ns []core.Status, err error) {
				err = withStore(func(s *core.Store) error { ns, err = s.Nudges(); return err })
				return ns, err
			},
			Resume: resumeDetached,
			Snooze: traySnooze,
			Drop:   func(id int64) error { return withStore(func(s *core.Store) error { return s.Drop(id) }) },
			Log:    func(err error) { fmt.Fprintf(os.Stderr, "proz tray: %v\n", err) },
		})
	},
}

// traySnooze snoozes a project from the tray's Snooze submenu.
func traySnooze(id int64, days int) error {
	return withStore(func(s *core.Store) error { return s.Snooze(id, time.Duration(days)*24*time.Hour) })
}

// trayEnabled reads the tray switch; if it cannot be read, keep running.
func trayEnabled() bool {
	on := true
	if err := withStore(func(s *core.Store) (err error) { on, err = s.ModuleOn(core.ModuleTray); return err }); err != nil {
		return true
	}
	return on
}

// resumeDetached opens a project from the tray, where there is no terminal:
// the program starts on its own, terminal programs in the user's terminal.
// A failure is also shown as a notification, so a launch is never silently
// lost. ponytail: first program only.
func resumeDetached(id int64) error {
	err := resumeTray(id)
	if err != nil {
		notify.Send([]notify.Message{{Title: "proz could not open the project", Body: err.Error()}})
	}
	return err
}

// resumeTray opens a project's first program, with no terminal attached.
func resumeTray(id int64) error {
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
		cmd, err = s.Launch(p, program, false)
		return err
	}); err != nil {
		return err
	}
	if cmd == nil {
		return fmt.Errorf("%s has no folder or program to open; run 'proz resume %s' in a terminal", p.Name, p.Name)
	}
	return startDetached(cmd)
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
