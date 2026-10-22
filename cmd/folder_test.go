package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/njayman/proz/core"
)

// folderJSON runs a folder command with --json and returns the project's
// path and its folder paths, main first.
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
		"folder add":                              "name the project",
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

// recorder puts a program on PATH that writes the folder it runs in to a
// file, and returns its name and a func that waits for that folder.
func recorder(t *testing.T) (name string, ranIn func() string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the recorder is a shell script")
	}
	bin, out := t.TempDir(), filepath.Join(t.TempDir(), "ran-in")
	if err := os.WriteFile(filepath.Join(bin, "proz-rec"), []byte("#!/bin/sh\npwd -P > "+out+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return "proz-rec", func() string {
		t.Helper()
		for range 200 { // started detached: wait for it
			if b, err := os.ReadFile(out); err == nil && len(b) > 0 {
				os.Remove(out)
				return strings.TrimSpace(string(b))
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("the program never ran")
		return ""
	}
}

func TestResumeOpensInTheChosenFolder(t *testing.T) {
	dir := configDir(t)
	main, docs, stray := filepath.Join(dir, "main"), filepath.Join(dir, "docs"), filepath.Join(dir, "stray")
	mkdirs(t, main, docs, stray)
	rec, ranIn := recorder(t)
	run(t, "idea", "web")
	run(t, "folder", "add", main, "--project", "web")
	run(t, "folder", "add", docs, "--project", "web")
	if err := withStore(func(s *core.Store) error {
		p, err := s.ProjectNamed("web")
		if err != nil {
			return err
		}
		p.Programs = []string{rec}
		return s.UpdateProject(p)
	}); err != nil {
		t.Fatal(err)
	}
	resolved := func(p string) string { r, _ := filepath.EvalSymlinks(p); return r }

	run(t, "resume", "web") // no terminal here: the main folder, no picker
	if got := ranIn(); got != resolved(main) {
		t.Fatalf("default: ran in %s, want %s", got, main)
	}
	run(t, "resume", "web", "--folder", docs)
	if got := ranIn(); got != resolved(docs) {
		t.Fatalf("--folder: ran in %s, want %s", got, docs)
	}
	run(t, "config", "terminal", rec)
	run(t, "resume", "web", "--folder", docs, "--terminal")
	if got := ranIn(); got != resolved(docs) {
		t.Fatalf("--folder --terminal: ran in %s, want %s", got, docs)
	}
	if err := execute("resume", "web", "--folder", stray); err == nil || !strings.Contains(err.Error(), "not a folder or worktree of web") {
		t.Fatalf("another folder: got %v", err)
	}
	run(t, "idea", "someday")
	if err := execute("resume", "someday", "--folder", docs); err == nil || !strings.Contains(err.Error(), "not a folder or worktree of someday") {
		t.Fatalf("--folder on an idea: got %v", err)
	}
}
