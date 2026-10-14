package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

// Responses to a nudge: resume, snooze, drop; plus archived and restore.

var resumeCmd = &cobra.Command{
	Use:   "resume [project]",
	Short: "Print where a project left off, then open it with its program",
	Long: "Print the project's left-off note, then launch its configured program\n" +
		"(asking which when there are several). With no name, uses the project\n" +
		"containing the current directory.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args)
		if err != nil {
			return err
		}
		var st core.Status
		if err := withStore(func(s *core.Store) (err error) {
			st, err = s.Status(p.ID)
			return err
		}); err != nil {
			return err
		}
		fmt.Println(st.Note.Full())
		if p.Path == "" {
			fmt.Fprintf(os.Stderr, "'%s' is an idea with no folder yet. Run 'proz add' in its folder to attach one.\n", p.Name)
			return launch(p, "") // still counts as picking it back up
		}
		return launch(p, chooseProgram(p))
	},
}

var snoozeCmd = &cobra.Command{
	Use:   "snooze [project] [days]",
	Short: "Hide a project's nudge for some days (default 7)",
	Long: "Hide a project's nudge for some days (default 7). If it is still quiet\n" +
		"when the snooze ends, it nudges again. With two arguments: project, days.\n" +
		"With one: a project name if one matches exactly, otherwise days.",
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		days := 7
		n := len(args)
		if n == 1 {
			if _, err := findProject(args[0]); err == nil {
				n = 0 // one argument that names a project is the project, even "2024"
			}
		}
		if n > 0 {
			if d, err := strconv.Atoi(args[n-1]); err == nil {
				if d < 1 || d > 36500 {
					return fmt.Errorf("days must be from 1 to 36500, got %d", d)
				}
				days, args = d, args[:n-1]
			}
		}
		return respond(args, func(s *core.Store, id int64) error {
			return s.Snooze(id, time.Duration(days)*24*time.Hour)
		}, fmt.Sprintf("Snoozed '%%s' for %d days.", days))
	},
}

var dropCmd = &cobra.Command{
	Use:   "drop [project]",
	Short: "Archive a project so it stops nudging",
	Long: "Archive a project so it stops nudging. Any new activity on it (a commit,\n" +
		"a launch, a milestone) brings it back automatically; so does 'proz restore'.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return respond(args, (*core.Store).Drop, "Dropped '%s'. New activity or 'proz restore' brings it back.")
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore [project]",
	Short: "Bring back an archived project and clear its snooze",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return respond(args, (*core.Store).Restore, "Restored '%s'.")
	},
}

var archivedJSON bool

var archivedCmd = &cobra.Command{
	Use:   "archived",
	Short: "List dropped projects",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var archived []core.Status
		if err := withStore(func(s *core.Store) (err error) {
			archived, err = s.Archived()
			return err
		}); err != nil {
			return err
		}
		if archivedJSON {
			out := []nudgeJSON{}
			for _, a := range archived {
				out = append(out, toNudgeJSON(a))
			}
			return printJSON(out)
		}
		for _, a := range archived {
			fmt.Printf("%s\t%s\t%s\n", a.Name, a.Note.Short(), a.Path)
		}
		return nil
	},
}

// respond resolves the project from args and applies one response to it.
func respond(args []string, fn func(*core.Store, int64) error, done string) error {
	p, err := resolveProject(args)
	if err != nil {
		return err
	}
	if err := withStore(func(s *core.Store) error { return fn(s, p.ID) }); err != nil {
		return err
	}
	fmt.Printf(done+"\n", p.Name)
	return nil
}

func init() {
	archivedCmd.Flags().BoolVar(&archivedJSON, "json", false, "print archived projects as JSON")
	rootCmd.AddCommand(resumeCmd, snoozeCmd, dropCmd, restoreCmd, archivedCmd)
}
