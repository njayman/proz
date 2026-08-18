package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long:  "Generate shell completion script. Detects the shell from $SHELL if no argument is given.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		shell := filepath.Base(os.Getenv("SHELL"))
		if len(args) > 0 {
			shell = args[0]
		}

		switch shell {
		case "zsh":
			rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			rootCmd.GenPowerShellCompletion(os.Stdout)
		default:
			rootCmd.GenBashCompletion(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
