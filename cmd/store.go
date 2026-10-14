package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/njayman/proz/core"
	"github.com/njayman/proz/utils"
)

type Project = core.Project

// withStore opens the core store for one operation. Opening per call keeps
// the CLI stateless; open cost is a few milliseconds.
func withStore(fn func(*core.Store) error) error {
	s, err := core.Open(utils.GetConfigFolder(), time.Now)
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

func loadProjects() (projects []Project, err error) {
	err = withStore(func(s *core.Store) error {
		projects, err = s.Projects()
		return err
	})
	return projects, err
}

func appendProject(project Project) {
	if err := withStore(func(s *core.Store) error { return s.AddProject(project) }); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving project: %v\n", err)
	}
}

func updateProject(project Project) error {
	return withStore(func(s *core.Store) error { return s.UpdateProject(project) })
}

func removeProject(id int64) error {
	return withStore(func(s *core.Store) error { return s.RemoveProject(id) })
}

func pushRecentProgram(program string) {
	if err := withStore(func(s *core.Store) error { return s.PushRecentProgram(program) }); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save recent program: %v\n", err)
	}
}

func loadRecentPrograms() (programs []string) {
	if err := withStore(func(s *core.Store) (err error) {
		programs, err = s.RecentPrograms()
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load recent programs: %v\n", err)
	}
	return programs
}

// launch opens project with program, attached to the terminal.
func launch(project Project, program string) error {
	var cmd *exec.Cmd
	if err := withStore(func(s *core.Store) (err error) {
		cmd, err = s.Launch(project, program)
		return err
	}); err != nil || cmd == nil {
		return err
	}
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer tty.Close()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	} else {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	}
	return cmd.Run()
}
