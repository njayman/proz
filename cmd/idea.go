package cmd

import (
	"fmt"
	"strings"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var ideaCmd = &cobra.Command{
	Use:   "idea <text>",
	Short: "Capture an idea as a project with no folder yet",
	Long: "Capture an idea in one line. It resurfaces like any project when it goes\n" +
		"quiet. Running 'proz add' later in a folder named like the idea offers to\n" +
		"attach that folder to it.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := strings.Join(args, " ")
		if err := withStore(func(s *core.Store) error { return s.AddIdea(text) }); err != nil {
			return err
		}
		fmt.Printf("Idea captured: %s\n", strings.Join(strings.Fields(text), " "))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ideaCmd)
}
