package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"github.com/njayman/proz/utils"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit [project-name]",
	Short: "Edit an existing project",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		utils.EnsureConfigFolderExists()

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
			selected, err := runProjectPicker(projects)
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

		for i := range projects {
			if projects[i].Name == target.Name && projects[i].Path == target.Path {
				projects[i] = *edited
				break
			}
		}

		fileData, err := json.MarshalIndent(projects, "", "  ")
		if err != nil {
			fmt.Printf("Error serializing projects: %v\n", err)
			return
		}
		if err := os.WriteFile(utils.GetConfigFilePath(), fileData, 0644); err != nil {
			fmt.Printf("Error writing to file: %v\n", err)
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

	fmt.Printf("Executable (%s): ", original.Executable)
	exec, _ := reader.ReadString('\n')
	exec = strings.TrimSpace(exec)
	if exec == "" {
		exec = original.Executable
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
		Name:       name,
		Path:       original.Path,
		Executable: exec,
		Arguments:  args,
	}
}
