package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/njayman/proz/utils"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var listJSON bool

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

		if listJSON {
			out, err := json.MarshalIndent(projects, "", "  ")
			if err != nil {
				fmt.Printf("Error serializing projects: %v\n", err)
				return
			}
			fmt.Println(string(out))
			return
		}

		if len(projects) == 0 {
			fmt.Println("no projects found")
			fmt.Println("add more projects using the add command")

			return
		}

		selected, addRequested, err := runProjectPicker(projects)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: TUI unavailable, showing text list")
			for i, p := range projects {
				fmt.Printf("[%d] %s (%s)\n", i+1, p.Name, p.Path)
			}
			return
		}
		if addRequested {
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current directory: %v\n", err)
				return
			}
			addFromPath(cwd)
			return
		}
		if selected == nil {
			return
		}

		if len(selected.Programs) == 0 {
			programs, err := runBinaryPicker(listPathBinaries())
			if err == nil && len(programs) > 0 {
				selected.Programs = programs
				for _, p := range programs {
					pushRecentExec(p)
				}
				updateProjectPrograms(*selected)
			}
		}

		program := ""
		switch len(selected.Programs) {
		case 0:
			// no program configured or chosen; just cd
		case 1:
			program = selected.Programs[0]
		default:
			choice, err := runProgramSelectPicker(selected.Programs)
			if err == nil {
				program = choice
			}
		}

		openProjectDetached(*selected, program)

		fmt.Println(selected.Path)
		fmt.Fprintln(os.Stderr, "Tip: use 'proz add'/'proz delete' to manage projects")
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "print projects as JSON instead of opening the picker")
	rootCmd.AddCommand(listCmd)
}

func openProjectDetached(project Project, program string) {
	if program == "" {
		return
	}

	pushRecentExec(program)

	os.Chdir(project.Path)
	cmd := exec.Command(program, append(project.Arguments, project.Path)...)
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

func updateProjectPrograms(project Project) {
	projects, err := loadProjects()
	if err != nil {
		return
	}
	for i := range projects {
		if projects[i].Name == project.Name && projects[i].Path == project.Path {
			projects[i].Programs = project.Programs
			break
		}
	}
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(utils.GetConfigFilePath(), data, 0644)
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
