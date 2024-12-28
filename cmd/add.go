package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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

var (
	projectPath string
	projectName string
	projectTags string
)

var addCmd = &cobra.Command{
	Use:   "add [project-name]",
	Short: "Add project directory to project lists",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]

		projectPath, err := os.Getwd()

		if err != nil {
			fmt.Printf("Error getting project directory")

			return
		}

		if projectName == "" {
			fmt.Println("project name not provided")
		}

		tags := []string{}

		if projectTags != "" {
			tags = strings.Split(projectTags, ",")
		}

		project := Project{Name: projectName, Path: projectPath, Tags: tags}
		utils.EnsureConfigFolderExists()
		saveProject(project)
		fmt.Printf("Project '%s' added successfully!\n", projectName)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&projectTags, "tags", "t", "", "Comma seperated tags for the project directory")
}

func saveProject(project Project) {
	dataFile := utils.GetConfigFilePath()

	var projects []Project

	if file, err := os.ReadFile(dataFile); err == nil {
		json.Unmarshal(file, &projects)
	}

	var execPath string

	fmt.Println("Enter the command/binary to open this project (e.g., code, nvim):")
	fmt.Scanln(&execPath)
	project.Executable = execPath

	var args string

	fmt.Print("Enter arguments for the command (space-separated, or press Enter for none): ")
	fmt.Scanln(&args)
	if args != "" {
		project.Arguments = strings.Fields(args)
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
