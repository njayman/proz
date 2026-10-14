package cmd

import (
	"errors"
	"fmt"
	"os"

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
		var p core.Project
		var err error
		if len(args) == 1 {
			p, err = findProject(args[0])
		} else {
			p, err = projectHere()
		}
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
		if statusJSON {
			return printJSON(struct {
				nudgeJSON
				Stale bool `json:"stale"`
			}{toNudgeJSON(st), st.Stale})
		}
		fmt.Println(st.Note.Full())
		return nil
	},
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
