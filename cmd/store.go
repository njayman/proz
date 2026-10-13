package cmd

import (
	"fmt"
	"os"
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

func pushRecentExec(program string) {
	withStore(func(s *core.Store) error { return s.PushRecentProgram(program) })
}

func loadRecentExecs() (execs []string) {
	withStore(func(s *core.Store) (err error) {
		execs, err = s.RecentPrograms()
		return err
	})
	return execs
}
