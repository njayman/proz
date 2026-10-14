package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit [project-name]",
	Short: "Edit a project's name, programs, or arguments",
	Long: "Edit a project's name, programs, or arguments. Without a name, opens\n" +
		"the project picker. Falls back to text prompts if the TUI is unavailable.",
	Args: cobra.MaximumNArgs(1),
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

		var target *Project
		if len(args) == 1 {
			for i := range projects {
				if projects[i].Name == args[0] {
					target = &projects[i]
					break
				}
			}
			if target == nil {
				fmt.Printf("Project '%s' not found\n", args[0])
				return
			}
		} else {
			selected, _, err := runProjectPicker(projects)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Warning: TUI unavailable")
				for i, p := range projects {
					fmt.Printf("[%d] %s (%s)\n", i+1, p.Name, p.Path)
				}
				return
			}
			if selected == nil {
				return
			}
			for i := range projects {
				if projects[i].Name == selected.Name && projects[i].Path == selected.Path {
					target = &projects[i]
					break
				}
			}
		}

		edited, err := runEditForm(*target)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: TUI unavailable, using text prompts")
			edited = editProjectText(target)
		}
		if edited == nil {
			return
		}

		edited.ID = target.ID
		if err := updateProject(*edited); err != nil {
			fmt.Printf("Error saving project: %v\n", err)
			return
		}
		fmt.Printf("Project '%s' updated successfully!\n", edited.Name)
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func editProjectText(original *Project) *Project {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Name (%s): ", original.Name)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = original.Name
	}

	currentPrograms := strings.Join(original.Programs, " ")
	fmt.Printf("Programs (%s): ", currentPrograms)
	progsStr, _ := reader.ReadString('\n')
	progsStr = strings.TrimSpace(progsStr)
	programs := original.Programs
	if progsStr != "" {
		programs = strings.Fields(progsStr)
	}

	currentArgs := strings.Join(original.Arguments, " ")
	fmt.Printf("Arguments (%s): ", currentArgs)
	argsStr, _ := reader.ReadString('\n')
	argsStr = strings.TrimSpace(argsStr)
	var args []string
	if argsStr == "" && currentArgs != "" {
		args = original.Arguments
	} else if argsStr != "" {
		args = strings.Fields(argsStr)
	}

	return &Project{
		Name:      name,
		Path:      original.Path,
		Programs:  programs,
		Arguments: args,
	}
}
