package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/njayman/proz/autostart"
	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or change proz settings",
}

var staleProject string

var staleDaysCmd = &cobra.Command{
	Use:   "stale-days [days]",
	Short: "Show or set how many quiet days make a project stale (default 7)",
	Long: "Show or set how many quiet days make a project stale (default 7).\n" +
		"With --project, show or set an override for one project; 0 removes it.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var p Project
		if cmd.Flags().Changed("project") && staleProject == "" {
			return fmt.Errorf("--project needs a project name")
		}
		if staleProject != "" {
			var err error
			if p, err = findProject(staleProject); err != nil {
				return err
			}
		}
		return withStore(func(s *core.Store) error {
			if len(args) == 1 {
				days, err := strconv.Atoi(args[0])
				if staleProject != "" && err == nil && days == 0 {
					return s.SetProjectStaleThreshold(p.ID, 0)
				}
				if err != nil || days < 1 || days > 36500 {
					return fmt.Errorf("days must be a whole number from 1 to 36500, got %q", args[0])
				}
				d := time.Duration(days) * 24 * time.Hour
				if staleProject != "" {
					return s.SetProjectStaleThreshold(p.ID, d)
				}
				return s.SetStaleThreshold(d)
			}
			d, err := s.StaleThreshold()
			if staleProject != "" {
				d, err = s.ProjectStaleThreshold(p.ID)
			}
			if err != nil {
				return err
			}
			fmt.Println(int(d / (24 * time.Hour)))
			return nil
		})
	},
}

var terminalJSON bool

var terminalCmd = &cobra.Command{
	Use:   "terminal [command]",
	Short: "Show or set the terminal proz opens terminal programs in",
	Long: "Show or set the terminal emulator proz opens terminal programs (nvim,\n" +
		"lazygit, claude, ...) in when it is not itself running in a terminal,\n" +
		"as from rofi or the tray. Empty means detect it. One word, like\n" +
		"'wezterm', uses that terminal's own form; several words, like\n" +
		"'wezterm start --', are used as written, with the program appended.\n" +
		"Set it to \"\" to go back to detecting.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error {
			if len(args) == 1 {
				if err := s.SetTerminal(args[0]); err != nil {
					return err
				}
			}
			t, err := s.Terminal()
			if err != nil {
				return err
			}
			if terminalJSON {
				return printJSON(map[string]string{"terminal": t})
			}
			if len(args) == 1 {
				return nil
			}
			if t == "" {
				t = "detect"
			}
			fmt.Println(t)
			return nil
		})
	},
}

// moduleCmd shows or switches one module on or off.
func moduleCmd(m core.Module) *cobra.Command {
	return &cobra.Command{
		Use:       string(m) + " [on|off]",
		Short:     "Show or switch " + string(m) + " on or off",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"on", "off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(func(s *core.Store) error {
				if len(args) == 1 {
					if args[0] != "on" && args[0] != "off" {
						return fmt.Errorf("%s: want on or off, got %q", m, args[0])
					}
					if m == core.ModuleTray && args[0] == "off" {
						// off means off: no tray at the next login either; a running one quits within 30 seconds
						if err := autostart.Remove(); err != nil {
							return err
						}
					}
					return s.SetModule(m, args[0] == "on")
				}
				on, err := s.ModuleOn(m)
				if err != nil {
					return err
				}
				if on {
					fmt.Println("on")
				} else {
					fmt.Println("off")
				}
				return nil
			})
		},
	}
}

func init() {
	for _, m := range core.Modules {
		configCmd.AddCommand(moduleCmd(m))
	}
	staleDaysCmd.Flags().StringVar(&staleProject, "project", "", "project name to override")
	configCmd.AddCommand(staleDaysCmd)
	terminalCmd.Flags().BoolVar(&terminalJSON, "json", false, "print the setting as JSON; an empty terminal means detect")
	configCmd.AddCommand(terminalCmd)
	rootCmd.AddCommand(configCmd)
}
