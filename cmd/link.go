package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var (
	linkProvider string
	linkKind     string
	linkJSONOut  bool
)

var linkCmd = &cobra.Command{
	Use:   "link <url-or-path> [project]",
	Short: "Link a project to the tracker or brain it uses",
	Long: "Link a project to where its tasks or notes live, such as its GitHub\n" +
		"repository or a Notion page, by pasting the URL. proz recognizes GitHub,\n" +
		"Linear, Jira, Trello, Asana and Todoist as trackers and Notion as a brain;\n" +
		"--kind tracker makes a Notion link a tracker. For any other address, say\n" +
		"what it is once with --provider and --kind; proz remembers it for that\n" +
		"host. owner/repo and owner/repo#123 mean GitHub, and an issue key such as\n" +
		"ENG-123 works once one Linear workspace or Jira site is linked. A file or\n" +
		"folder of notes is a brain, named obsidian, logseq or folder from what is\n" +
		"on disk, and editing it counts as activity. proz never\n" +
		"reads through a link; agents do. With no project name, uses the project\n" +
		"containing the current folder.",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args[1:])
		if err != nil {
			return err
		}
		var l core.Link
		err = changeProject(p.ID, linkJSONOut, func(s *core.Store) (err error) {
			l, err = s.AddLink(p.ID, linkRef(args[0]), linkProvider, linkKind)
			return err
		}, func(after core.Project) string {
			return fmt.Sprintf("Linked %s to %s, as a %s: %s", after.Name, linkName(l), l.Kind, l.Ref)
		})
		var unknown *core.UnknownHostError
		if errors.As(err, &unknown) {
			return fmt.Errorf("proz does not recognize %s; say which tool it is with --provider %s and --kind %s|%s",
				unknown.Host, strings.Join(core.Providers(), "|"), core.LinkTracker, core.LinkBrain)
		}
		return err
	},
}

// linkRef is arg as core takes it: a file or folder that exists here, even
// one named like owner/repo, as an absolute path; anything else as written.
func linkRef(arg string) string {
	if _, err := os.Stat(arg); err == nil {
		if abs, err := filepath.Abs(arg); err == nil {
			return abs
		}
	}
	return arg
}

// linkName is how a person tells l apart: its provider, or for a web link
// (provider other) its domain.
func linkName(l core.Link) string {
	if u, err := url.Parse(l.Ref); err == nil && l.Provider == "web" {
		return u.Host
	}
	return l.Provider
}

var unlinkCmd = &cobra.Command{
	Use:   "unlink <url-or-path> [project]",
	Short: "Remove a link from a project",
	Long: "Remove a link from a project; other projects with the same link keep\n" +
		"it. With no project name, uses the project containing the current folder.",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args[1:])
		if err != nil {
			return err
		}
		return changeProject(p.ID, linkJSONOut, func(s *core.Store) error { return s.RemoveLink(p.ID, linkRef(args[0])) },
			func(after core.Project) string { return fmt.Sprintf("Unlinked %s from %s.", args[0], after.Name) })
	},
}

func init() {
	linkCmd.Flags().StringVar(&linkProvider, "provider", "", "which tool it is, for an address proz does not recognize: "+strings.Join(core.Providers(), ", "))
	linkCmd.Flags().StringVar(&linkKind, "kind", "", "tracker (where its tasks live) or brain (where its notes live); defaults to the tool's usual kind")
	for _, c := range []*cobra.Command{linkCmd, unlinkCmd} {
		c.Flags().BoolVar(&linkJSONOut, "json", false, "print the project as JSON")
		rootCmd.AddCommand(c)
	}
}
