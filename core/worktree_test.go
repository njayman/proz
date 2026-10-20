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

func TestAWorktreeInsideAnotherProjectsFolderFindsItsOwnProject(t *testing.T) {
	s := open(t, t.TempDir())
	repo, commit := gitRepo(t)
	commit(t0)
	dev := t.TempDir()
	wt := filepath.Join(dev, "app-spike")
	git(t, repo, t0, "worktree", "add", "-q", "-b", "spike", wt)
	if err := s.AddProject(Project{Name: "dev", Path: dev}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddProject(Project{Name: "app", Path: repo}); err != nil {
		t.Fatal(err)
	}
	if p, err := s.ProjectAt(wt); err != nil || p.Name != "app" {
		t.Fatalf("got %q, %v; want app, whose checkout the worktree is", p.Name, err)
	}
	if p, err := s.ProjectAt(dev); err != nil || p.Name != "dev" {
		t.Fatalf("dev itself: got %q, %v", p.Name, err)
	}
}

func TestACommitInAWorktreeIsActivity(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	repo, commit := gitRepo(t)
	commit(t0)
	wt := worktree(t, repo, "spike")
	if err := s.AddProject(Project{Name: "app", Path: repo}); err != nil {
		t.Fatal(err)
	}
	touch(t, wt, "spike.go", t0)
	git(t, wt, t0, "add", "-A")
	git(t, wt, t0.Add(6*day), "commit", "-q", "-m", "spike works")

	c.t = t0.Add(20 * day)
	st, err := s.Status(projectNamed(t, s, "app").ID)
	if err != nil || !st.LastActivity.Equal(t0.Add(6*day)) || !strings.Contains(st.Note.Full(), "Last commit: spike works") {
		t.Fatalf("last activity %v, note %q, %v", st.LastActivity, st.Note.Lines, err)
	}
}

func TestABareRepositoryIsNotAPlaceToRead(t *testing.T) {
	repo, commit := gitRepo(t)
	commit(t0)
	bare := filepath.Join(t.TempDir(), "bare.git")
	git(t, repo, t0, "clone", "-q", "--bare", repo, bare)
	wt := filepath.Join(t.TempDir(), "main")
	git(t, bare, t0, "worktree", "add", "-q", "-b", "work", wt)
	for _, p := range otherCheckouts(t.Context(), wt) {
		if resolve(p) == resolve(bare) {
			t.Fatalf("the bare repository is listed: %v", otherCheckouts(t.Context(), wt))
		}
	}
}

func TestLaunchOpensInTheChosenFolderOrWorktreeAndRefusesOthers(t *testing.T) {
	s := open(t, t.TempDir())
	repo, commit := gitRepo(t)
	commit(t0)
	wt := worktree(t, repo, "spike")
	docs := t.TempDir()
	if err := s.AddProject(Project{Name: "app", Path: repo}); err != nil {
		t.Fatal(err)
	}
	app := projectNamed(t, s, "app")
	if err := s.AddFolder(app.ID, docs); err != nil {
		t.Fatal(err)
	}
	app = projectNamed(t, s, "app")

	for folder, want := range map[string]string{"": repo, docs: docs, wt: wt} {
		cmd, err := s.Launch(app, folder, "code", true)
		if err != nil || cmd.Dir != want || cmd.Args[len(cmd.Args)-1] != want {
			t.Fatalf("folder %q: got %v, %v; want %s", folder, cmd, err, want)
		}
		if cmd, err = s.LaunchTerminal(app, folder, true); err != nil || cmd.Dir != want {
			t.Fatalf("terminal in %q: got %v, %v", folder, cmd, err)
		}
	}
	stray := t.TempDir()
	if _, err := s.Launch(app, stray, "code", true); err == nil || !strings.Contains(err.Error(), "not a folder or worktree of app") {
		t.Fatalf("another folder: got %v", err)
	}
	if _, err := s.LaunchTerminal(app, stray, true); err == nil || !strings.Contains(err.Error(), "not a folder or worktree of app") {
		t.Fatalf("a terminal in another folder: got %v", err)
	}
}

func TestCheckoutsListFoldersAndWorktreesMostRecentlyActiveFirst(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	repo, commit := gitRepo(t)
	commit(t0)
	wt := worktree(t, repo, "spike")
	docs := t.TempDir()
	if err := s.AddProject(Project{Name: "app", Path: repo}); err != nil {
		t.Fatal(err)
	}
	app := projectNamed(t, s, "app")
	if err := s.AddFolder(app.ID, docs); err != nil {
		t.Fatal(err)
	}
	touch(t, docs, "plan.md", t0.Add(2*day))
	touch(t, wt, "spike.go", t0.Add(5*day))

	c.t = t0.Add(20 * day)
	got, err := s.Checkouts(app)
	if err != nil {
		t.Fatal(err)
	}
	want := []Checkout{
		{Path: wt, Worktree: true, LastActive: t0.Add(5 * day)},
		{Path: docs, LastActive: t0.Add(2 * day)},
		{Path: repo, Main: true, LastActive: t0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if resolve(got[i].Path) != resolve(want[i].Path) || got[i].Main != want[i].Main || got[i].Worktree != want[i].Worktree || !got[i].LastActive.Equal(want[i].LastActive) {
			t.Fatalf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
