package cmd

import (
	"fmt"

	"github.com/njayman/proz/contract"
	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var brainJSON bool

var brainCmd = &cobra.Command{
	Use:   "brain [project]",
	Short: "Print a project's note from the brain folder",
	Long: "Bring a project's note in the brain folder up to date and print it, your\n" +
		"own text included. With no name, uses the project that contains the\n" +
		"current directory. Set the folder with: proz config brain-path <dir>",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := resolveProject(args)
		if err != nil {
			return err
		}
		var note contract.BrainNote
		if err := withStore(func(s *core.Store) (err error) {
			note.Path, note.Content, err = s.BrainNote(p.ID)
			return err
		}); err != nil {
			return err
		}
		if brainJSON {
			return printJSON(note)
		}
		fmt.Print(note.Content)
		return nil
	},
}

var brainPathCmd = &cobra.Command{
	Use:   "brain-path [dir]",
	Short: "Show or set the brain: a markdown folder where proz keeps one note per project",
	Long: "Show or set the brain: a local markdown folder (an Obsidian vault works)\n" +
		"where proz keeps one note per project with its left-off note and\n" +
		"milestones. proz only rewrites the section between its markers, so the\n" +
		"rest of each note is yours. Setting the folder switches the brain on.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error {
			if len(args) == 1 {
				if err := s.SetBrainPath(args[0]); err != nil {
					return err
				}
				return s.SyncBrain()
			}
			dir, err := s.BrainPath()
			if err != nil {
				return err
			}
			if dir == "" {
				dir = "not set"
			}
			fmt.Println(dir)
			return nil
		})
	},
}

func init() {
	brainCmd.Flags().BoolVar(&brainJSON, "json", false, "print the note's path and content as JSON")
	configCmd.AddCommand(brainPathCmd)
	rootCmd.AddCommand(brainCmd)
}
