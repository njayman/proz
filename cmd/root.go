package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "proz",
	Short: "proz is a free & open source cli tool for managing project directories",
	Long:  "proz is a free & open source cli tool for managing project directories",
	Run: func(cmd *cobra.Command, args []string) {
		listCmd.Run(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occured while executing proz '%s'\n", err)
		os.Exit(1)
	}
}
