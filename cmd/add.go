package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/njayman/proz/utils"
	"github.com/spf13/cobra"
)

type Project struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Programs  []string `json:"programs,omitempty"`
	Arguments []string `json:"arguments"`
}

// UnmarshalJSON migrates the old single "executable" field into Programs.
func (p *Project) UnmarshalJSON(data []byte) error {
	type Alias Project
	aux := struct {
		Executable string `json:"executable"`
		*Alias
	}{Alias: (*Alias)(p)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(p.Programs) == 0 && aux.Executable != "" {
		p.Programs = []string{aux.Executable}
	}
	return nil
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

func addFromPath(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: '%s' is not a directory\n", path)
		os.Exit(1)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	name := filepath.Base(absPath)
	project := Project{Name: name, Path: absPath}
	utils.EnsureConfigFolderExists()
	if err := saveProject(project); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Project '%s' added successfully!\n", name)
}

func saveProject(project Project) error {
	binaries := listPathBinaries()
	programs, err := runBinaryPicker(binaries)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: TUI unavailable — saving without a program")
		fmt.Fprintln(os.Stderr, "  Use 'proz edit' to set one later")
	} else if len(programs) > 0 {
		project.Programs = programs
		for _, p := range programs {
			pushRecentExec(p)
		}
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
