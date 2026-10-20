package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations)-4)); err != nil {
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
