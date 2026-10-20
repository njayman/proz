package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [project-name]",
	Short: "Add the current directory as a project",
	Long: "Add the current directory as a project.\n\n" +
		"Opens a program picker (desktop apps by default, Tab for all PATH binaries;\n" +
		"space toggles multiple programs, q/esc skips it). Then prompts for arguments.",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		path, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error getting project directory: %v\n", err)
			return
		}

		project := Project{Name: name, Path: path}
		if err := register(project); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
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
	if err := register(project); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
}

func saveProject(project Project) error {
	binaries := listPathBinaries()
	programs, err := runBinaryPicker(binaries)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: TUI unavailable, saving without a program")
		fmt.Fprintln(os.Stderr, "  Use 'proz edit' to set one later")
	} else if len(programs) > 0 {
		project.Programs = programs
		for _, p := range programs {
			pushRecentProgram(p)
		}
	}

	fmt.Print("Arguments (space-separated, or press Enter for none): ")
	argsStr, err := stdin.ReadString('\n')
	if err == nil {
		argsStr = strings.TrimSpace(argsStr)
		if argsStr != "" {
			project.Arguments = strings.Fields(argsStr)
		}
	}

	if project.ID != 0 {
		return updateProject(project)
	}
	appendProject(project)
	return nil
}

// stdin is shared so successive prompts don't lose each other's buffered input.
var stdin = bufio.NewReader(os.Stdin)

// register saves project, or, if its folder is named like an existing idea
// and the user agrees, gives that idea the folder so one idea never becomes
// two projects. With no answer (no terminal, as for agents) it attaches.
// A folder or worktree another project has is added only once confirmed.
func register(project Project) error {
	// A folder or worktree another project has becomes a second project
	// only when the user says so; with no answer nothing is added.
	var claimed *core.ClaimedError
	if err := withStore(func(s *core.Store) error { return s.Claim(project.Path) }); errors.As(err, &claimed) {
		fmt.Printf("%v.\nTrack it as a separate project anyway? [y/N] ", claimed)
		answer, _ := stdin.ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			fmt.Println("Not added.")
			return nil
		}
	} else if err != nil {
		return err
	}
	var idea Project
	if err := withStore(func(s *core.Store) (err error) {
		idea, err = s.IdeaFor(project.Path)
		return err
	}); err == nil {
		fmt.Printf("Attach this folder to the idea %q instead of adding a new project? [Y/n] ", idea.Name)
		answer, _ := stdin.ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a == "" || a == "y" || a == "yes" {
			idea.Path = project.Path
			project = idea
		}
	} else if !errors.Is(err, core.ErrNotFound) {
		return err
	}
	if err := saveProject(project); err != nil {
		return err
	}
	if project.ID != 0 {
		fmt.Printf("Folder attached to the idea '%s'.\n", project.Name)
	} else {
		fmt.Printf("Project '%s' added successfully!\n", project.Name)
	}
	offerGitHubLink(project)
	return nil
}

// offerGitHubLink asks whether to link the GitHub repository the folder's
// origin points at as the project's tracker. Enter or yes links it; anything
// else, or no answer at all (no terminal, as for agents), never does.
func offerGitHubLink(project Project) {
	ref := core.GitHubOrigin(project.Path)
	if ref == "" || slices.ContainsFunc(project.Links, func(l core.Link) bool { return l.Ref == ref }) {
		return
	}
	fmt.Printf("Link its GitHub issues (%s) as the project's tracker? [Y/n] ", ref)
	answer, err := stdin.ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); err != nil || (a != "" && a != "y" && a != "yes") {
		fmt.Println("Not linked.")
		return
	}
	if err := withStore(func(s *core.Store) error {
		projects, err := s.Projects()
		if err != nil {
			return err
		}
		id := project.ID
		for _, p := range slices.Backward(projects) { // a new project is the newest one in its folder
			if id == 0 && p.Name == project.Name && p.Path == project.Path {
				id = p.ID
				break
			}
		}
		_, err = s.AddLink(id, ref, "", "")
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not link %s: %v\n", ref, err)
		return
	}
	fmt.Printf("Linked %s to %s.\n", project.Name, ref)
}
