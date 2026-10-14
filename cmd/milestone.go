package cmd

import (
	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var milestoneCmd = &cobra.Command{
	Use:   "milestone [project] <text>",
	Short: "Record a progress checkpoint on a project",
	Long: "Record a milestone, such as \"shipped login; next: signup\". The latest\n" +
		"milestone leads the project's left-off note and counts as activity.\n" +
		"With no project name, uses the project containing the current directory.",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args[:len(args)-1])
		if err != nil {
			return err
		}
		return withStore(func(s *core.Store) error { return s.AddMilestone(p.ID, args[len(args)-1]) })
	},
}

func init() {
	rootCmd.AddCommand(milestoneCmd)
}
