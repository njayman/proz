package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/njayman/proz/utils"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [project-name]",
	Short: "Add the current directory as a project",
	Long: "Add the current directory as a project.\n\n" +
		"Opens a program picker (desktop apps by default, Tab for all PATH binaries;\n" +
		"space toggles multiple programs, q/esc skips it). Then prompts for arguments.",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		path, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error getting project directory: %v\n", err)
			return
		}

		project := Project{Name: name, Path: path}
		utils.EnsureConfigFolderExists()
		if err := saveProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		fmt.Printf("Project '%s' added successfully!\n", name)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func addFromPath(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: '%s' is not a directory\n", path)
		os.Exit(1)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	name := filepath.Base(absPath)
	project := Project{Name: name, Path: absPath}
	utils.EnsureConfigFolderExists()
	if err := saveProject(project); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Project '%s' added successfully!\n", name)
}

func saveProject(project Project) error {
	binaries := listPathBinaries()
	programs, err := runBinaryPicker(binaries)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: TUI unavailable, saving without a program")
		fmt.Fprintln(os.Stderr, "  Use 'proz edit' to set one later")
	} else if len(programs) > 0 {
		project.Programs = programs
		for _, p := range programs {
			pushRecentExec(p)
		}
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Arguments (space-separated, or press Enter for none): ")
	argsStr, err := reader.ReadString('\n')
	if err == nil {
		argsStr = strings.TrimSpace(argsStr)
		if argsStr != "" {
			project.Arguments = strings.Fields(argsStr)
		}
	}

	appendProject(project)
	return nil
}
