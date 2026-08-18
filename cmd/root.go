package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

var rootCmd = &cobra.Command{
	Use:   "proz [path]",
	Short: "proz is a free & open source cli tool for managing project directories",
	Long:  "proz is a free & open source cli tool for managing project directories",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			addFromPath(args[0])
			return
		}
		listCmd.Run(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occured while executing proz '%s'\n", err)
		os.Exit(1)
	}
}
