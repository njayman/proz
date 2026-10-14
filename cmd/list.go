package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	listJSON   bool
	listFilter string
	listSort   string
	listLimit  int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Open the project picker",
	Long: "Open the project picker. Selecting a project launches its configured\n" +
		"program(s) in that directory (prompts if more than one is configured).\n" +
		"The picker's bottom row adds the current directory as a new project.\n" +
		"--json prints all projects as JSON instead.\n" +
		"--filter, --sort, and --limit apply to both the picker and --json output.",
	Run: func(cmd *cobra.Command, args []string) {

		projects, err := loadProjects()

		if err != nil {
			fmt.Printf("Error loading projects: %v\n", err)

			return
		}

		projects, err = applyListOptions(projects)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
			if listFilter != "" {
				fmt.Printf("no projects match filter %q\n", listFilter)
			} else {
				fmt.Println("no projects found")
				fmt.Println("add more projects using the add command")
			}

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
		if selected.Path == "" {
			launch(*selected, "") // opening an idea still counts as activity
			fmt.Fprintf(os.Stderr, "'%s' is an idea with no folder yet. Run 'proz add' in its folder to attach one.\n", selected.Name)
			return
		}

		if len(selected.Programs) == 0 {
			programs, err := runBinaryPicker(listPathBinaries())
			if err == nil && len(programs) > 0 {
				selected.Programs = programs
				for _, p := range programs {
					pushRecentProgram(p)
				}
				if err := updateProject(*selected); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving programs: %v\n", err)
				}
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

		if err := launch(*selected, program); err != nil {
			fmt.Fprintf(os.Stderr, "Error launching %s: %v\n", program, err)
		}

		fmt.Println(selected.Path)
		fmt.Fprintln(os.Stderr, "Tip: use 'proz add'/'proz delete' to manage projects")
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "print projects as JSON instead of opening the picker")
	listCmd.Flags().StringVar(&listFilter, "filter", "", "keep projects whose name or path contains this substring (case-insensitive)")
	listCmd.Flags().StringVar(&listSort, "sort", "", "sort projects: name or path")
	listCmd.Flags().IntVar(&listLimit, "limit", 0, "limit the number of projects returned (0 = no limit)")
	rootCmd.AddCommand(listCmd)
}

func applyListOptions(projects []Project) ([]Project, error) {
	if listFilter != "" {
		lower := strings.ToLower(listFilter)
		filtered := make([]Project, 0, len(projects))
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Name), lower) || strings.Contains(strings.ToLower(p.Path), lower) {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	switch listSort {
	case "":
		// keep stored order
	case "name":
		sort.SliceStable(projects, func(i, j int) bool {
			return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
		})
	case "path":
		sort.SliceStable(projects, func(i, j int) bool {
			return strings.ToLower(projects[i].Path) < strings.ToLower(projects[j].Path)
		})
	default:
		return nil, fmt.Errorf("invalid --sort value %q, must be 'name' or 'path'", listSort)
	}

	if listLimit > 0 && listLimit < len(projects) {
		projects = projects[:listLimit]
	}

	return projects, nil
}
