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

func TestOpenProjectDetached_EmptyExec(t *testing.T) {
	openProjectDetached(Project{Name: "test", Path: "/nonexistent"})
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	setupTestDir(t)
	p := Project{Name: "rt", Path: "/tmp", Executable: "echo", Arguments: []string{"hello"}}
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

func TestLoadRecentExecs_FileNotExist(t *testing.T) {
	setupTestDir(t)
	execs := loadRecentExecs()
	if execs != nil {
		t.Fatalf("expected nil for missing file, got %v", execs)
	}
}

func TestPushRecentExec_AddsFirst(t *testing.T) {
	setupTestDir(t)
	pushRecentExec("code")
	execs := loadRecentExecs()
	if len(execs) != 1 || execs[0] != "code" {
		t.Fatalf("expected [code], got %v", execs)
	}
}

func TestPushRecentExec_Deduplicates(t *testing.T) {
	setupTestDir(t)
	pushRecentExec("code")
	pushRecentExec("code")
	execs := loadRecentExecs()
	if len(execs) != 1 {
		t.Fatalf("expected 1 after dedup, got %v", execs)
	}
}

func TestPushRecentExec_MovesToFront(t *testing.T) {
	setupTestDir(t)
	pushRecentExec("vim")
	pushRecentExec("code")
	pushRecentExec("nvim")
	pushRecentExec("code")
	execs := loadRecentExecs()
	if len(execs) != 3 {
		t.Fatalf("expected 3 items, got %v", execs)
	}
	if execs[0] != "code" {
		t.Fatalf("expected 'code' first, got %v", execs)
	}
}

func TestPushRecentExec_MaxFour(t *testing.T) {
	setupTestDir(t)
	pushRecentExec("a")
	pushRecentExec("b")
	pushRecentExec("c")
	pushRecentExec("d")
	pushRecentExec("e")
	execs := loadRecentExecs()
	if len(execs) != 4 {
		t.Fatalf("expected 4 items max, got %d: %v", len(execs), execs)
	}
	if execs[3] != "b" {
		t.Fatalf("expected 'b' as oldest kept, got %v", execs)
	}
}

func TestSaveRecentExecs_RoundTrip(t *testing.T) {
	setupTestDir(t)
	err := saveRecentExecs([]string{"nvim", "code"})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	execs := loadRecentExecs()
	if len(execs) != 2 || execs[0] != "nvim" || execs[1] != "code" {
		t.Fatalf("round-trip failed: %v", execs)
	}
}
