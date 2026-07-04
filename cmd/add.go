package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/njayman/proz/utils"
)

type Project struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
}

var addCmd = &cobra.Command{
	Use:   "add [project-name]",
	Short: "Add project directory to project lists",
	Args:  cobra.ExactArgs(1),
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

func saveProject(project Project) error {
	binaries := listPathBinaries()
	bin, err := runBinaryPicker(binaries)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: TUI unavailable — saving without executable")
		fmt.Fprintln(os.Stderr, "  Use 'proz edit' to set an executable later")
	} else if bin != "" {
		project.Executable = bin
		pushRecentExec(bin)
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

func appendProject(project Project) {
	utils.EnsureConfigFolderExists()
	dataFile := utils.GetConfigFilePath()

	var projects []Project
	if file, err := os.ReadFile(dataFile); err == nil {
		json.Unmarshal(file, &projects)
	}

	projects = append(projects, project)
	fileData, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		fmt.Printf("Error serializing projects: %v\n", err)
		return
	}
	if err := os.WriteFile(dataFile, fileData, 0644); err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
	}
}
