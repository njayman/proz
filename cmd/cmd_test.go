package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "proz", "store.json")
}

func writeTestProjects(t *testing.T, projects []Project) string {
	t.Helper()
	path := setupTestDir(t)
	os.MkdirAll(filepath.Dir(path), 0775)
	data, _ := json.MarshalIndent(projects, "", "  ")
	os.WriteFile(path, data, 0644)
	return path
}

func TestLoadProjects_FileNotExist(t *testing.T) {
	setupTestDir(t)
	projects, err := loadProjects()
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected empty projects, got %d", len(projects))
	}
}

func TestLoadProjects_ValidFile(t *testing.T) {
	expected := []Project{{Name: "test", Path: "/tmp"}}
	writeTestProjects(t, expected)
	projects, err := loadProjects()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "test" {
		t.Fatalf("expected 1 project named 'test', got %+v", projects)
	}
}

func TestLoadProjects_CorruptFile(t *testing.T) {
	path := setupTestDir(t)
	os.MkdirAll(filepath.Dir(path), 0775)
	os.WriteFile(path, []byte("not json}"), 0644)
	_, err := loadProjects()
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestLoadProjects_MultipleProjects(t *testing.T) {
	projects := []Project{
		{Name: "a", Path: "/a"},
		{Name: "b", Path: "/b"},
	}
	writeTestProjects(t, projects)
	result, err := loadProjects()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(result))
	}
}

func TestHasAnyTag_Match(t *testing.T) {
	if !hasAnyTag([]string{"go", "cli"}, []string{"cli"}) {
		t.Fatal("expected match")
	}
}

func TestHasAnyTag_NoMatch(t *testing.T) {
	if hasAnyTag([]string{"go", "rust"}, []string{"python"}) {
		t.Fatal("expected no match")
	}
}

func TestHasAnyTag_CaseInsensitive(t *testing.T) {
	if !hasAnyTag([]string{"Go"}, []string{"go"}) {
		t.Fatal("expected case-insensitive match")
	}
}

func TestHasAnyTag_EmptyFilter(t *testing.T) {
	if hasAnyTag([]string{"go"}, []string{""}) {
		t.Fatal("expected no match for empty filter")
	}
}

func TestHasAnyTag_EmptyProjectTags(t *testing.T) {
	if hasAnyTag([]string{}, []string{"go"}) {
		t.Fatal("expected no match for empty project tags")
	}
}

func TestHasAnyTag_TrimSpaces(t *testing.T) {
	if !hasAnyTag([]string{"go", "cli"}, []string{" go "}) {
		t.Fatal("expected match with trimmed spaces")
	}
}

func TestOpenProjectDetached_EmptyExec(t *testing.T) {
	openProjectDetached(Project{Name: "test", Path: "/nonexistent"})
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	setupTestDir(t)
	p := Project{Name: "rt", Path: "/tmp", Executable: "echo", Arguments: []string{"hello"}, Tags: []string{"test"}}
	appendProject(p)
	projects, err := loadProjects()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "rt" || projects[0].Executable != "echo" {
		t.Fatalf("round-trip failed: %+v", projects[0])
	}
}

func TestSaveAndLoadAppends(t *testing.T) {
	setupTestDir(t)
	appendProject(Project{Name: "a", Path: "/a"})
	appendProject(Project{Name: "b", Path: "/b"})
	projects, err := loadProjects()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects after two saves, got %d", len(projects))
	}
}
