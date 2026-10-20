package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/njayman/proz/autostart"
	"github.com/njayman/proz/contract"
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
				return printJSON(contract.TerminalSetting{Terminal: t})
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

var themeJSON bool

var themeCmd = &cobra.Command{
	Use:   "theme [system|light|dark]",
	Short: "Show or set the launcher's light or dark look",
	Long: "Show or set how the launcher (proz gui) looks: system follows the\n" +
		"system's light or dark mode, light and dark always use that one.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error {
			if len(args) == 1 {
				if err := s.SetTheme(core.Theme(args[0])); err != nil {
					return err
				}
			}
			t, err := s.Theme()
			if err != nil {
				return err
			}
			if themeJSON {
				return printJSON(contract.ThemeSetting{Theme: string(t)})
			}
			if len(args) == 0 {
				fmt.Println(t)
			}
			return nil
		})
	},
}

var needsTerminalJSON bool

var needsTerminalCmd = &cobra.Command{
	Use:   "needs-terminal <program> [on|off|auto]",
	Short: "Show or set whether a program opens in a terminal",
	Long: "Show or set whether proz opens a program in a terminal when it is not\n" +
		"itself running in one. proz guesses from the program's .desktop entry\n" +
		"and a list of well-known terminal programs; on or off overrides the\n" +
		"guess, auto goes back to it.",
	Args:      cobra.RangeArgs(1, 2),
	ValidArgs: []string{"on", "off", "auto"},
	RunE: func(cmd *cobra.Command, args []string) error {
		program := args[0]
		return withStore(func(s *core.Store) error {
			if len(args) == 2 {
				var on *bool
				switch args[1] {
				case "on", "off":
					v := args[1] == "on"
					on = &v
				case "auto":
				default:
					return fmt.Errorf("want on, off or auto, got %q", args[1])
				}
				if err := s.SetNeedsTerminal(program, on); err != nil {
					return err
				}
			}
			need, err := s.NeedsTerminal(program)
			if err != nil {
				return err
			}
			if needsTerminalJSON {
				return printJSON(contract.NeedsTerminal{Program: program, NeedsTerminal: need.On, Set: need.Set})
			}
			if len(args) == 2 {
				return nil
			}
			v, how := "off", "guessed"
			if need.On {
				v = "on"
			}
			if need.Set {
				how = "set"
			}
			fmt.Printf("%s (%s)\n", v, how)
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
	themeCmd.Flags().BoolVar(&themeJSON, "json", false, "print the setting as JSON")
	configCmd.AddCommand(themeCmd)
	needsTerminalCmd.Flags().BoolVar(&needsTerminalJSON, "json", false, "print program, needs_terminal and set (true when it is your setting) as JSON")
	configCmd.AddCommand(needsTerminalCmd)
	rootCmd.AddCommand(configCmd)
}
