package core

import (
	"path/filepath"
	"strings"
	"testing"
)

// worktree adds a worktree of repo on a new branch and returns its folder.
func worktree(t *testing.T, repo, branch string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), branch)
	git(t, repo, t0, "worktree", "add", "-q", "-b", branch, dir)
	return dir
}

func TestWorkInAWorktreeCountsForTheProject(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	repo, commit := gitRepo(t)
	commit(t0)
	wt := worktree(t, repo, "spike")
	touch(t, wt, "idea.md", t0.Add(4*day))
	if err := s.AddProject(Project{Name: "app", Path: repo}); err != nil {
		t.Fatal(err)
	}

	c.t = t0.Add(20 * day)
	st, err := s.Status(projectNamed(t, s, "app").ID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.LastActivity.Equal(t0.Add(4*day)) || !strings.Contains(st.Note.Full(), "Branch: spike") || !strings.Contains(st.Note.Full(), "Uncommitted: 1 file") {
		t.Fatalf("last activity %v, note %q", st.LastActivity, st.Note.Lines)
	}
	if len(st.Folders) != 1 {
		t.Fatalf("a worktree is never stored as a folder: %+v", st.Folders)
	}
}

func TestAddingAWorktreeAsAFolderIsRefusedNamingItsProject(t *testing.T) {
	s := open(t, t.TempDir())
	repo, commit := gitRepo(t)
	commit(t0)
	wt := worktree(t, repo, "spike")
	if err := s.AddProject(Project{Name: "app", Path: repo}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIdea("other"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"other", "app"} {
		err := s.AddFolder(projectNamed(t, s, name).ID, wt)
		if err == nil || !strings.Contains(err.Error(), "app") {
			t.Fatalf("adding the worktree to %s: got %v, want an error naming app", name, err)
		}
	}
}

func TestRunningInsideAWorktreeFindsTheProject(t *testing.T) {
	s := open(t, t.TempDir())
	repo, commit := gitRepo(t)
	gitCommit(t, repo, "src/main.go", t0)
	commit(t0)
	wt := worktree(t, repo, "spike")
	if err := s.AddProject(Project{Name: "app", Path: repo}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{wt, filepath.Join(wt, "src")} {
		if p, err := s.ProjectAt(dir); err != nil || p.Name != "app" {
			t.Fatalf("%s: got %q, %v", dir, p.Name, err)
		}
	}
}
