package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func projectNamed(t *testing.T, s *Store, name string) Project {
	t.Helper()
	p, err := s.ProjectNamed(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAProjectsFolderIsItsMainFolderAndAnIdeaHasNone(t *testing.T) {
	s := open(t, t.TempDir())
	dir := t.TempDir()
	if err := s.AddProject(Project{Name: "web", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIdea("habit tracker"); err != nil {
		t.Fatal(err)
	}

	web := projectNamed(t, s, "web")
	if want := []Folder{{Path: dir, Main: true}}; !reflect.DeepEqual(web.Folders, want) || web.Path != dir {
		t.Fatalf("web: path %q, folders %+v", web.Path, web.Folders)
	}
	idea := projectNamed(t, s, "habit tracker")
	if idea.Folders == nil || len(idea.Folders) != 0 || idea.Path != "" {
		t.Fatalf("idea: path %q, folders %#v", idea.Path, idea.Folders)
	}
}

func TestUpgradingMakesEachProjectsFolderItsMainFolder(t *testing.T) {
	dir, web := t.TempDir(), t.TempDir()
	s := open(t, dir)
	if err := s.AddProject(Project{Name: "web", Path: web}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIdea("habit tracker"); err != nil {
		t.Fatal(err)
	}
	// wind the store back to before folders had a table of their own
	if _, err := s.db.Exec(`DROP TABLE folders`); err != nil {
		t.Fatal(err)
	}
	before := slices.IndexFunc(migrations, func(m string) bool { return strings.HasPrefix(m, "CREATE TABLE folders") })
	if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, before)); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s = open(t, dir)
	p := projectNamed(t, s, "web")
	if want := []Folder{{Path: web, Main: true}}; !reflect.DeepEqual(p.Folders, want) || p.Path != web {
		t.Fatalf("web: path %q, folders %+v", p.Path, p.Folders)
	}
	if idea := projectNamed(t, s, "habit tracker"); len(idea.Folders) != 0 {
		t.Fatalf("idea folders %+v", idea.Folders)
	}
	st, err := s.Status(p.ID)
	if err != nil || st.Path != web {
		t.Fatalf("status path %q, %v", st.Path, err)
	}
}

func TestAFolderIsMissingWhenGoneOrUnderAFileButNotWhenUnreadable(t *testing.T) {
	root := t.TempDir()
	gone := filepath.Join(root, "gone")
	underFile := filepath.Join(root, "file", "web")
	locked := filepath.Join(root, "locked", "api")
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	s := open(t, t.TempDir())
	for name, dir := range map[string]string{"gone": gone, "under-file": underFile, "locked": locked} {
		if err := s.AddProject(Project{Name: name, Path: dir}); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS != "windows" { // no unix permissions there
		os.Chmod(filepath.Dir(locked), 0)
		t.Cleanup(func() { os.Chmod(filepath.Dir(locked), 0o755) })
	}

	for name, want := range map[string]bool{"gone": true, "under-file": true, "locked": false} {
		if got := projectNamed(t, s, name).Folders[0].Missing; got != want {
			t.Errorf("%s: missing %v, want %v", name, got, want)
		}
	}
}

func TestGivingAnIdeaAFolderMakesItTheMainFolder(t *testing.T) {
	s := open(t, t.TempDir())
	if _, err := s.AddIdea("habit tracker"); err != nil {
		t.Fatal(err)
	}
	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		p := projectNamed(t, s, "habit tracker")
		p.Path = dir
		if err := s.UpdateProject(p); err != nil {
			t.Fatal(err)
		}
		p = projectNamed(t, s, "habit tracker")
		if want := []Folder{{Path: dir, Main: true}}; !reflect.DeepEqual(p.Folders, want) || p.Path != dir {
			t.Fatalf("path %q, folders %+v", p.Path, p.Folders)
		}
	}
}

func TestAFolderAnOlderProzWroteIsPickedUpOnOpen(t *testing.T) {
	dir, web, api := t.TempDir(), t.TempDir(), t.TempDir()
	s := open(t, dir)
	if err := s.AddProject(Project{Name: "web", Path: web}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIdea("api"); err != nil {
		t.Fatal(err)
	}
	// an older proz only knows projects.path
	if _, err := s.db.Exec(`UPDATE projects SET path = CASE name WHEN 'web' THEN ? ELSE ? END`, api+"-moved", api); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s = open(t, dir)
	for name, want := range map[string]string{"web": api + "-moved", "api": api} {
		p := projectNamed(t, s, name)
		if len(p.Folders) != 1 || p.Folders[0].Path != want || !p.Folders[0].Main || p.Path != want {
			t.Errorf("%s: path %q, folders %+v", name, p.Path, p.Folders)
		}
	}
}

// folders lists p's folder paths, main first, and its path.
func folderPaths(t *testing.T, s *Store, name string) (main string, all []string) {
	t.Helper()
	p := projectNamed(t, s, name)
	for _, f := range p.Folders {
		all = append(all, f.Path)
		if f.Main && f.Path != p.Path {
			t.Fatalf("%s: main folder %q but path %q", name, f.Path, p.Path)
		}
	}
	return p.Path, all
}

func TestTheFirstFolderAddedToAnIdeaIsItsMainFolder(t *testing.T) {
	s := open(t, t.TempDir())
	if _, err := s.AddIdea("habit tracker"); err != nil {
		t.Fatal(err)
	}
	idea := projectNamed(t, s, "habit tracker")
	if err := s.AddMilestone(idea.ID, "sketched screens"); err != nil {
		t.Fatal(err)
	}
	backend, frontend := t.TempDir(), t.TempDir()
	for _, dir := range []string{backend, frontend} {
		if err := s.AddFolder(idea.ID, dir); err != nil {
			t.Fatal(err)
		}
	}

	main, all := folderPaths(t, s, "habit tracker")
	if main != backend || !reflect.DeepEqual(all, []string{backend, frontend}) {
		t.Fatalf("main %q, folders %v", main, all)
	}
	if ms, err := s.Milestones(idea.ID); err != nil || len(ms) != 1 {
		t.Fatalf("milestones %v, %v", ms, err)
	}
}

func TestAFolderAnotherProjectOwnsIsRefusedNamingIt(t *testing.T) {
	s := open(t, t.TempDir())
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if err := s.AddProject(Project{Name: "web", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIdea("api"); err != nil {
		t.Fatal(err)
	}
	api := projectNamed(t, s, "api")
	for _, d := range []string{dir, link} {
		if err := s.AddFolder(api.ID, d); err == nil || !strings.Contains(err.Error(), "web") {
			t.Fatalf("adding %s: got %v, want an error naming web", d, err)
		}
	}
	if _, all := folderPaths(t, s, "api"); len(all) != 0 {
		t.Fatalf("api folders %v", all)
	}
}

func TestRemovingTheMainFolderPromotesTheNextOldestAndTheLastMakesAnIdea(t *testing.T) {
	s := open(t, t.TempDir())
	a, b, c := t.TempDir(), t.TempDir(), t.TempDir()
	if err := s.AddProject(Project{Name: "web", Path: a}); err != nil {
		t.Fatal(err)
	}
	web := projectNamed(t, s, "web")
	for _, d := range []string{b, c} {
		if err := s.AddFolder(web.ID, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RemoveFolder(web.ID, t.TempDir()); err == nil {
		t.Fatal("removing a folder the project does not have must fail")
	}

	if err := s.RemoveFolder(web.ID, a); err != nil {
		t.Fatal(err)
	}
	if main, all := folderPaths(t, s, "web"); main != b || !reflect.DeepEqual(all, []string{b, c}) {
		t.Fatalf("after removing main: main %q, folders %v", main, all)
	}
	for _, d := range []string{c, b} {
		if err := s.RemoveFolder(web.ID, d); err != nil {
			t.Fatal(err)
		}
	}
	if main, all := folderPaths(t, s, "web"); main != "" || len(all) != 0 {
		t.Fatalf("after removing all: main %q, folders %v", main, all)
	}
}

func TestChoosingTheMainFolder(t *testing.T) {
	s := open(t, t.TempDir())
	a, b := t.TempDir(), t.TempDir()
	if err := s.AddProject(Project{Name: "web", Path: a}); err != nil {
		t.Fatal(err)
	}
	web := projectNamed(t, s, "web")
	if err := s.AddFolder(web.ID, b); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMainFolder(web.ID, t.TempDir()); err == nil {
		t.Fatal("a folder the project does not have cannot be its main folder")
	}
	if err := s.SetMainFolder(web.ID, b); err != nil {
		t.Fatal(err)
	}
	if main, all := folderPaths(t, s, "web"); main != b || !reflect.DeepEqual(all, []string{b, a}) {
		t.Fatalf("main %q, folders %v", main, all)
	}
}

func TestEditingThePathToAnotherOfItsFoldersMakesThatTheMainOne(t *testing.T) {
	s := open(t, t.TempDir())
	a, b := t.TempDir(), t.TempDir()
	if err := s.AddProject(Project{Name: "web", Path: a}); err != nil {
		t.Fatal(err)
	}
	web := projectNamed(t, s, "web")
	if err := s.AddFolder(web.ID, b); err != nil {
		t.Fatal(err)
	}
	web = projectNamed(t, s, "web")
	web.Path = b
	if err := s.UpdateProject(web); err != nil {
		t.Fatal(err)
	}
	if main, all := folderPaths(t, s, "web"); main != b || !reflect.DeepEqual(all, []string{b}) {
		t.Fatalf("main %q, folders %v", main, all)
	}
}

func TestTheDeepestFolderAcrossAllProjectsFindsTheProject(t *testing.T) {
	s := open(t, t.TempDir())
	a, b := t.TempDir(), t.TempDir()
	nested := filepath.Join(b, "tools")
	if err := os.MkdirAll(filepath.Join(nested, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.AddProject(Project{Name: "web", Path: a}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIdea("tools"); err != nil {
		t.Fatal(err)
	}
	web, tools := projectNamed(t, s, "web"), projectNamed(t, s, "tools")
	if err := s.AddFolder(web.ID, b); err != nil {
		t.Fatal(err)
	}
	if err := s.AddFolder(tools.ID, nested); err != nil {
		t.Fatalf("a folder inside another project's folder is allowed: %v", err)
	}

	for dir, want := range map[string]string{b: "web", filepath.Join(nested, "src"): "tools", a: "web"} {
		p, err := s.ProjectAt(dir)
		if err != nil || p.Name != want {
			t.Errorf("%s: got %q, %v; want %s", dir, p.Name, err, want)
		}
	}
}
