package cmd

import (
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Alias for delete",
	Run:   deleteCmd.Run,
	Args:  deleteCmd.Args,
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
