package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit [project-name]",
	Short: "Edit a project's name, programs, or arguments",
	Long: "Edit a project's name, programs, or arguments. Without a name, opens\n" +
		"the project picker. Falls back to text prompts if the TUI is unavailable.\n" +
		"With --name or --program it changes just those, without prompting.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if f := cmd.Flags(); f.Changed("name") || f.Changed("program") {
			if len(args) != 1 {
				return errors.New("--name and --program need a project name")
			}
			return editWithFlags(args[0], f.Changed("name"), f.Changed("program"))
		}

		projects, err := loadProjects()
		if err != nil {
			fmt.Printf("Error loading projects: %v\n", err)
			return nil
		}

		if len(projects) == 0 {
			fmt.Println("no projects found")
			return nil
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
				return nil
			}
		} else {
			selected, _, err := runProjectPicker(projects)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Warning: TUI unavailable")
				for i, p := range projects {
					fmt.Printf("[%d] %s (%s)\n", i+1, p.Name, p.Path)
				}
				return nil
			}
			if selected == nil {
				return nil
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
			return nil
		}

		edited.ID = target.ID
		if err := updateProject(*edited); err != nil {
			fmt.Printf("Error saving project: %v\n", err)
			return nil
		}
		fmt.Printf("Project '%s' updated successfully!\n", edited.Name)
		return nil
	},
}

var (
	editName     string
	editPrograms []string
)

// editWithFlags renames the project named name and sets its programs, as
// the flags given say, for agents and scripts that have no terminal.
func editWithFlags(name string, rename, programs bool) error {
	p, err := findProject(name)
	if err != nil {
		return err
	}
	if rename {
		if p.Name = strings.Join(strings.Fields(editName), " "); p.Name == "" {
			return errors.New("--name is empty")
		}
	}
	if programs {
		p.Programs = slices.DeleteFunc(editPrograms, func(s string) bool { return s == "" }) // --program "" clears them
	}
	if err := updateProject(p); err != nil {
		return err
	}
	fmt.Printf("Project '%s' updated.\n", p.Name)
	return nil
}

func init() {
	editCmd.Flags().StringVar(&editName, "name", "", "rename the project")
	editCmd.Flags().StringArrayVar(&editPrograms, "program", nil, "a program it opens in, repeated for several; --program \"\" clears them")
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
