package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var (
	folderProject string
	folderJSONOut bool
)

var folderCmd = &cobra.Command{
	Use:   "folder",
	Short: "Add, remove or choose the main folder of a project",
	Long: "A project can have several folders, such as a backend and a frontend.\n" +
		"The main folder is the one proz opens by default. A project with no\n" +
		"folders is an idea.",
}

var folderAddCmd = &cobra.Command{
	Use:   "add [folder]",
	Short: "Add a folder to a project (the current folder by default)",
	Long: "Add a folder to the project named by --project, or else the project\n" +
		"containing the current folder. The first folder is the main folder, so\n" +
		"adding one to an idea makes it a project.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && folderProject == "" { // the current folder is already inside the project it would find
			return errors.New("name the project to add this folder to: proz folder add --project <name>")
		}
		dir, err := folderArg(args)
		if err != nil {
			return err
		}
		p, err := folderTarget("")
		if err != nil {
			return err
		}
		return changeProject(p.ID, folderJSONOut, func(s *core.Store) error { return s.AddFolder(p.ID, dir) },
			func(after core.Project) string {
				if after.Path == dir {
					return fmt.Sprintf("Added %s to %s as its main folder.", dir, after.Name)
				}
				return fmt.Sprintf("Added %s to %s.", dir, after.Name)
			})
	},
}

var folderRmCmd = &cobra.Command{
	Use:   "rm <folder>",
	Short: "Remove a folder from a project",
	Long: "Remove a folder from its project, or from the project named by\n" +
		"--project. The folder itself is left alone. Removing the main folder\n" +
		"makes the next oldest the main one; removing the last makes the\n" +
		"project an idea again, its milestones kept.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := folderArg(args)
		if err != nil {
			return err
		}
		p, err := folderTarget(dir)
		if err != nil {
			return err
		}
		return changeProject(p.ID, folderJSONOut, func(s *core.Store) error { return s.RemoveFolder(p.ID, dir) },
			func(after core.Project) string {
				if after.Path == "" {
					return fmt.Sprintf("Removed %s; %s has no folders and is an idea again.", dir, after.Name)
				}
				return fmt.Sprintf("Removed %s from %s; its main folder is %s.", dir, after.Name, after.Path)
			})
	},
}

var folderMainCmd = &cobra.Command{
	Use:   "main <folder>",
	Short: "Make one of a project's folders its main folder",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := folderArg(args)
		if err != nil {
			return err
		}
		p, err := folderTarget(dir)
		if err != nil {
			return err
		}
		return changeProject(p.ID, folderJSONOut, func(s *core.Store) error { return s.SetMainFolder(p.ID, dir) },
			func(after core.Project) string {
				return fmt.Sprintf("%s is now the main folder of %s.", after.Path, after.Name)
			})
	},
}

// folderArg is the folder in args as an absolute path, or the current one.
func folderArg(args []string) (string, error) {
	if len(args) == 0 {
		return os.Getwd()
	}
	return filepath.Abs(args[0])
}

// folderTarget is the project named by --project; else the project that has
// dir as a folder; else, with no dir, the project containing the current folder.
func folderTarget(dir string) (core.Project, error) {
	if folderProject != "" {
		return findProject(folderProject)
	}
	if dir == "" {
		return projectHere()
	}
	var p core.Project
	err := withStore(func(s *core.Store) (err error) { p, err = s.FolderOwner(dir); return err })
	if errors.Is(err, core.ErrNotFound) {
		return p, fmt.Errorf("%s is not a folder of any project", dir)
	}
	return p, err
}

func init() {
	folderCmd.PersistentFlags().StringVar(&folderProject, "project", "", "the project's name")
	folderCmd.PersistentFlags().BoolVar(&folderJSONOut, "json", false, "print the project as JSON")
	folderCmd.AddCommand(folderAddCmd, folderRmCmd, folderMainCmd)
	rootCmd.AddCommand(folderCmd)
}
