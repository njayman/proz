package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/njayman/proz/core"
)

// folderJSON runs a folder command with --json and returns the project's
// folders as main-first paths, and its path.
func folderJSON(t *testing.T, args ...string) (path string, folders []string) {
	t.Helper()
	var p struct {
		Name    string        `json:"name"`
		Path    string        `json:"path"`
		Folders []core.Folder `json:"folders"`
	}
	if err := json.Unmarshal([]byte(run(t, append(args, "--json")...)), &p); err != nil {
		t.Fatal(err)
	}
	for _, f := range p.Folders {
		folders = append(folders, f.Path)
	}
	return p.Path, folders
}

func TestFolderCommandsAddRemoveAndChooseTheMainFolder(t *testing.T) {
	dir := configDir(t)
	backend, frontend := filepath.Join(dir, "backend"), filepath.Join(dir, "frontend")
	mkdirs(t, backend, frontend)
	run(t, "idea", "habit", "tracker")

	t.Chdir(backend)
	if path, fs := folderJSON(t, "folder", "add", "--project", "habit tracker"); path != backend || !reflect.DeepEqual(fs, []string{backend}) {
		t.Fatalf("add here: path %q, folders %v", path, fs)
	}
	if path, fs := folderJSON(t, "folder", "add", "../frontend"); path != backend || !reflect.DeepEqual(fs, []string{backend, frontend}) {
		t.Fatalf("add relative from inside the project: path %q, folders %v", path, fs)
	}
	t.Chdir(dir)
	if path, fs := folderJSON(t, "folder", "main", "frontend"); path != frontend || !reflect.DeepEqual(fs, []string{frontend, backend}) {
		t.Fatalf("main: path %q, folders %v", path, fs)
	}
	if path, fs := folderJSON(t, "folder", "rm", frontend); path != backend || !reflect.DeepEqual(fs, []string{backend}) {
		t.Fatalf("rm main: path %q, folders %v", path, fs)
	}
	if path, fs := folderJSON(t, "folder", "rm", backend, "--project", "habit tracker"); path != "" || len(fs) != 0 {
		t.Fatalf("rm last: path %q, folders %v", path, fs)
	}
}

func TestFolderCommandErrorsSayFolder(t *testing.T) {
	dir := configDir(t)
	web, stray := filepath.Join(dir, "web"), filepath.Join(dir, "stray")
	mkdirs(t, web, stray)
	t.Chdir(web)
	run(t, "idea", "web")
	run(t, "folder", "add", "--project", "web")
	run(t, "idea", "api")

	for args, want := range map[string]string{
		"folder add " + web + " --project api":    "already a folder of web",
		"folder rm " + stray:                      "not a folder of any project",
		"folder main " + stray + " --project web": "not a folder of web",
	} {
		err := execute(strings.Fields(args)...)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want %q", args, err, want)
		}
	}
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
