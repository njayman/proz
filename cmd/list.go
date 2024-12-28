package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"proz/utils"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "lists the project directory lists",
	Run: func(cmd *cobra.Command, args []string) {
		utils.EnsureConfigFolderExists()

		projects, err := loadProjects()

		if err != nil {
			fmt.Printf("Error loading projects: %v\n", err)

			return
		}

		if len(projects) == 0 {
			fmt.Println("no projects found")
			fmt.Println("add more projects using the add command")

			return
		}

		fmt.Println("Stored projects")
		for i, project := range projects {
			fmt.Printf("[%d] %s (Path: %s, Tags: %v)\n", i+1, project.Name, project.Path, project.Tags)
		}

		var choice int

		fmt.Print("Select a project to open by number: ")
		fmt.Scanln(&choice)

		if choice < 1 || choice > len(projects) {
			fmt.Println("Invalid selection")

			return
		}

		selectedProject := projects[choice-1]

		if err := openProject(selectedProject); err != nil {
			fmt.Printf("Error opening project: %v\n", err)
		}

	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func openProject(project Project) error {
	if project.Executable == "" {
		if err := os.Chdir(project.Path); err != nil {
			return fmt.Errorf("failed to change directory to '%s': %w", project.Path, err)
		}

		fmt.Printf("Changed directory to: %s\n", project.Path)

		return nil
	}

	cmd := exec.Command(project.Executable, append(project.Arguments, project.Path)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Opening project '%s' with command: %s %v\n", project.Name, project.Executable, project.Arguments)

	return cmd.Run()
}

func loadProjects() ([]Project, error) {
	dataFile := utils.GetConfigFilePath()

	var projects []Project

	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		return projects, nil
	}

	file, err := os.ReadFile(dataFile)

	if err != nil {
		return nil, fmt.Errorf("failed to read data config file: %w\n", err)
	}

	if err := json.Unmarshal(file, &projects); err != nil {
		return nil, fmt.Errorf("failed to parse project data. Data tile may be corrupted or broken. %w", err)
	}

	return projects, nil
}
