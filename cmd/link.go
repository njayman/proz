package cmd

import (
	"fmt"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var (
	linkKind    string
	linkJSONOut bool
)

var linkCmd = &cobra.Command{
	Use:   "link <url> [project]",
	Short: "Link a project to the tracker or brain it uses",
	Long: "Link a project to where its tasks or notes live, such as its GitHub\n" +
		"repository or a Notion page, by pasting the URL. proz recognizes GitHub,\n" +
		"Linear, Jira, Trello, Asana and Todoist as trackers and Notion as a brain;\n" +
		"--kind tracker makes a Notion link a tracker. proz never reads through a\n" +
		"link; agents do. With no project name, uses the project containing the\n" +
		"current folder.",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args[1:])
		if err != nil {
			return err
		}
		var l core.Link
		return changeProject(p.ID, linkJSONOut, func(s *core.Store) (err error) { l, err = s.AddLink(p.ID, args[0], linkKind); return err },
			func(after core.Project) string {
				return fmt.Sprintf("Linked %s to %s, as a %s: %s", after.Name, l.Provider, l.Kind, l.Ref)
			})
	},
}

var unlinkCmd = &cobra.Command{
	Use:   "unlink <url> [project]",
	Short: "Remove a link from a project",
	Long: "Remove a link from a project; other projects with the same link keep\n" +
		"it. With no project name, uses the project containing the current folder.",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args[1:])
		if err != nil {
			return err
		}
		return changeProject(p.ID, linkJSONOut, func(s *core.Store) error { return s.RemoveLink(p.ID, args[0]) },
			func(after core.Project) string { return fmt.Sprintf("Unlinked %s from %s.", args[0], after.Name) })
	},
}

func init() {
	linkCmd.Flags().StringVar(&linkKind, "kind", "", "tracker (where its tasks live) or brain (where its notes live); defaults to the tool's usual kind")
	for _, c := range []*cobra.Command{linkCmd, unlinkCmd} {
		c.Flags().BoolVar(&linkJSONOut, "json", false, "print the project as JSON")
		rootCmd.AddCommand(c)
	}
}
