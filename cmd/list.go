package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"proz/utils"

	"github.com/spf13/cobra"
)

var listTags string

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

		if listTags != "" {
			filterTags := strings.Split(listTags, ",")
			var filtered []Project
			for _, p := range projects {
				if hasAnyTag(p.Tags, filterTags) {
					filtered = append(filtered, p)
				}
			}
			projects = filtered
		}

		if len(projects) == 0 {
			fmt.Println("no projects found")
			fmt.Println("add more projects using the add command")

			return
		}

		selected, err := runProjectPicker(projects)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: TUI unavailable, showing text list")
			for i, p := range projects {
				fmt.Printf("[%d] %s (Path: %s, Tags: %v)\n", i+1, p.Name, p.Path, p.Tags)
			}
			return
		}
		if selected == nil {
			return
		}

		openProjectDetached(*selected)

		fmt.Println(selected.Path)
		fmt.Fprintln(os.Stderr, "Tip: use 'proz add'/'proz delete' to manage projects")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listTags, "tags", "t", "", "Filter projects by comma-separated tags")
}

func openProjectDetached(project Project) {
	if project.Executable == "" {
		return
	}

	os.Chdir(project.Path)
	cmd := exec.Command(project.Executable, append(project.Arguments, project.Path)...)
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer tty.Close()
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	cmd.Run()
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
