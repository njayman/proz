package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status [project]",
	Short: "Show where a project left off",
	Long: "Print a project's left-off note. With no name, uses the project that\n" +
		"contains the current directory (including subfolders).",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args)
		if err != nil {
			return err
		}
		var st core.Status
		var milestones []core.Milestone
		if err := withStore(func(s *core.Store) (err error) {
			if st, err = s.Status(p.ID); err != nil {
				return err
			}
			milestones, err = s.Milestones(p.ID)
			return err
		}); err != nil {
			return err
		}
		if statusJSON {
			type milestoneJSON struct {
				Text string `json:"text"`
				At   string `json:"at"`
			}
			ms := []milestoneJSON{}
			for _, m := range milestones {
				ms = append(ms, milestoneJSON{m.Text, m.At.UTC().Format(time.RFC3339)})
			}
			return printJSON(struct {
				nudgeJSON
				Stale      bool            `json:"stale"`
				Milestones []milestoneJSON `json:"milestones"`
			}{toNudgeJSON(st), st.Stale, ms})
		}
		fmt.Println(st.Note.Full())
		if len(milestones) > 0 {
			fmt.Println("\nMilestones:")
			for _, m := range milestones {
				fmt.Printf("%s  %s\n", m.At.UTC().Format(time.DateOnly), strings.ReplaceAll(m.Text, "\n", "\n            "))
			}
		}
		return nil
	},
}

// resolveProject is the project named by args[0], or with no args the one
// containing the current directory.
func resolveProject(args []string) (core.Project, error) {
	if len(args) > 0 {
		return findProject(args[0])
	}
	return projectHere()
}

// projectHere is the project containing the current directory.
func projectHere() (p core.Project, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return p, err
	}
	err = withStore(func(s *core.Store) (err error) {
		p, err = s.ProjectAt(cwd)
		return err
	})
	if errors.Is(err, core.ErrNotFound) {
		return p, fmt.Errorf("%s is not inside a proz project; name one, or register it with 'proz add'", cwd)
	}
	return p, err
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "print status as JSON")
	rootCmd.AddCommand(statusCmd)
}
