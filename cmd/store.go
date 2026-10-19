package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/njayman/proz/core"
	"github.com/njayman/proz/utils"
)

type Project = core.Project

// now is the clock core reads; tests replace it.
var now = time.Now

// withStore opens the core store for one operation. Opening per call keeps
// the CLI stateless; open cost is a few milliseconds.
func withStore(fn func(*core.Store) error) error {
	s, err := core.Open(utils.GetConfigFolder(), now)
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

// launch opens project with program. In a terminal the program runs there;
// without one (rofi, wofi) core opens terminal programs in the user's terminal.
func launch(project Project, program string) error {
	tty, ttyErr := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if ttyErr == nil {
		defer tty.Close()
	}
	attached := ttyErr == nil || term.IsTerminal(os.Stdin.Fd())
	var cmd *exec.Cmd
	if err := withStore(func(s *core.Store) (err error) {
		cmd, err = s.Launch(project, program, attached)
		return err
	}); err != nil || cmd == nil {
		return err
	}
	if !attached {
		return startDetached(cmd)
	}
	if tty != nil {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	} else {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	}
	return cmd.Run()
}

// startDetached starts cmd on its own and reaps it when it exits, so no
// zombie lingers.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// findProject returns the first project named name.
func findProject(name string) (p Project, err error) {
	err = withStore(func(s *core.Store) (err error) {
		p, err = s.ProjectNamed(name)
		return err
	})
	if errors.Is(err, core.ErrNotFound) {
		err = fmt.Errorf("project %q not found", name)
	}
	return p, err
}
