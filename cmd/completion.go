package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate shell completion script",
	Run: func(cmd *cobra.Command, args []string) {
		shell := filepath.Base(os.Getenv("SHELL"))
		var name, rc string

		switch shell {
		case "zsh":
			rootCmd.GenZshCompletion(os.Stdout)
			name, rc = "zsh", "~/.zshrc"
		case "fish":
			rootCmd.GenFishCompletion(os.Stdout, true)
			name, rc = "fish", "~/.config/fish/config.fish"
		case "powershell":
			rootCmd.GenPowerShellCompletion(os.Stdout)
			name, rc = "PowerShell", "your PowerShell profile"
		default:
			rootCmd.GenBashCompletion(os.Stdout)
			name, rc = "bash", "~/.bashrc"
		}

		fmt.Fprintf(os.Stderr, "\n# To enable %s completions, add to %s:\n", name, rc)
		fmt.Fprintf(os.Stderr, "#   source <(proz completion)\n")
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
