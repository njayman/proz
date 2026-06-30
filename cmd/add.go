package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"proz/utils"
)

type Project struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
	Tags       []string `json:"tags"`
}

var projectTags string

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

		tags := []string{}
		if projectTags != "" {
			for _, t := range strings.Split(projectTags, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		project := Project{Name: name, Path: path, Tags: tags}
		utils.EnsureConfigFolderExists()
		saveProject(project)
		fmt.Printf("Project '%s' added successfully!\n", name)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&projectTags, "tags", "t", "", "Comma seperated tags for the project directory")
}

func saveProject(project Project) {
	var execPath string
	fmt.Println("Enter the command/binary to open this project (e.g., code, nvim):")
	fmt.Scanln(&execPath)
	if execPath != "" {
		if _, err := exec.LookPath(execPath); err != nil {
			fmt.Printf("Warning: '%s' not found in PATH\n", execPath)
		}
	}
	project.Executable = execPath

	var args string
	fmt.Print("Enter arguments for the command (space-separated, or press Enter for none): ")
	fmt.Scanln(&args)
	if args != "" {
		project.Arguments = strings.Fields(args)
	}

	appendProject(project)
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
