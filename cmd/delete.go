package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"github.com/njayman/proz/utils"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [project-name]",
	Short: "Delete a project from the list",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projects, err := loadProjects()
		if err != nil {
			fmt.Printf("Error loading projects: %v\n", err)
			return
		}

		if len(projects) == 0 {
			fmt.Println("no projects found")
			return
		}

		var idx int
		if len(args) == 1 {
			found := -1
			for i, p := range projects {
				if p.Name == args[0] {
					found = i
					break
				}
			}
			if found == -1 {
				fmt.Printf("Project '%s' not found\n", args[0])
				return
			}
			idx = found
		} else {
			selected, err := runProjectPicker(projects)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Warning: TUI unavailable, showing text list")
				for i, p := range projects {
					fmt.Printf("[%d] %s (%s)\n", i+1, p.Name, p.Path)
				}
				return
			}
			if selected == nil {
				return
			}
			for i, p := range projects {
				if p.Name == selected.Name && p.Path == selected.Path {
					idx = i
					break
				}
			}
		}

		name := projects[idx].Name
		projects = append(projects[:idx], projects[idx+1:]...)

		fileData, err := json.MarshalIndent(projects, "", "  ")
		if err != nil {
			fmt.Printf("Error serializing projects: %v\n", err)
			return
		}
		if err := os.WriteFile(utils.GetConfigFilePath(), fileData, 0644); err != nil {
			fmt.Printf("Error writing to file: %v\n", err)
			return
		}
		fmt.Printf("Project '%s' deleted successfully!\n", name)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
