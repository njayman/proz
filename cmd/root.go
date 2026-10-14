package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SilenceUsage = true  // a failed command's error is not a usage mistake
	rootCmd.SilenceErrors = true // Execute prints it once
}

var rootCmd = &cobra.Command{
	Use:   "proz [path]",
	Short: "Store project directories and launch them with configured programs",
	Long: "Store project directories and launch them with configured programs.\n\n" +
		"With no arguments, opens the project picker (same as 'proz list').\n" +
		"With a path argument, adds that directory as a project (same as 'proz add' but skips cd).",
	Args: cobra.MaximumNArgs(1),
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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
