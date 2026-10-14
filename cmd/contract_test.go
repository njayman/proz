package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// run executes proz with args and returns what it printed to stdout.
func run(t *testing.T, args ...string) string {
	t.Helper()
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	os.Stdout = stdout
	w.Close()
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// Agents parse `proz list --json`; field names and shape are a contract.
func TestListJSONContract(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[
  {"name": "web", "path": "/web", "programs": ["code", "claude"], "arguments": ["--new-window"]},
  {"name": "notes", "path": "/notes", "arguments": []},
  {"name": "legacy", "path": "/old", "executable": "nvim"}
]`), 0644)

	out := run(t, "list", "--json")

	want := `[
  {
    "name": "web",
    "path": "/web",
    "programs": [
      "code",
      "claude"
    ],
    "arguments": [
      "--new-window"
    ]
  },
  {
    "name": "notes",
    "path": "/notes",
    "arguments": []
  },
  {
    "name": "legacy",
    "path": "/old",
    "programs": [
      "nvim"
    ],
    "arguments": null
  }
]
`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// Agents parse `proz nudges --json`; field names and shape are a contract.
func TestNudgesJSONContract(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "/web"}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { now = time.Now })

	now = func() time.Time { return t0 }
	if out := run(t, "nudges", "--json"); out != "[]\n" {
		t.Fatalf("fresh project: got %q", out)
	}

	now = func() time.Time { return t0.Add(10*24*time.Hour + time.Hour) }
	want := `[
  {
    "name": "web",
    "path": "/web",
    "last_activity": "2026-10-01T09:00:00Z",
    "quiet_days": 10,
    "quiet_seconds": 867600,
    "note": "Quiet for 10 days",
    "short_note": "Quiet for 10 days"
  }
]
`
	if out := run(t, "nudges", "--json"); out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestConfigStaleDays(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if out := run(t, "config", "stale-days"); out != "7\n" {
		t.Fatalf("default: got %q", out)
	}
	rootCmd.SetArgs([]string{"config", "stale-days", "300000"})
	if rootCmd.Execute() == nil {
		t.Fatal("a threshold that overflows a duration should be rejected")
	}
	run(t, "config", "stale-days", "14")
	if out := run(t, "config", "stale-days"); out != "14\n" {
		t.Fatalf("after set: got %q", out)
	}
}

func TestConfigProjectStaleDays(t *testing.T) {
	t.Cleanup(func() { staleProject = "" }) // cobra keeps flag values between Execute calls
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "/web"}, {"name": "api", "path": "/api"}]`), 0644)

	run(t, "config", "stale-days", "30", "--project", "web")
	if out := run(t, "config", "stale-days", "--project", "web"); out != "30\n" {
		t.Fatalf("override: got %q", out)
	}
	if out := run(t, "config", "stale-days", "--project", "api"); out != "7\n" {
		t.Fatalf("no override shows global: got %q", out)
	}
	run(t, "config", "stale-days", "0", "--project", "web")
	if out := run(t, "config", "stale-days", "--project", "web"); out != "7\n" {
		t.Fatalf("cleared: got %q", out)
	}
	for _, name := range []string{"nope", ""} {
		rootCmd.SetArgs([]string{"config", "stale-days", "5", "--project", name})
		if rootCmd.Execute() == nil {
			t.Fatalf("project %q should fail", name)
		}
	}
	if out := run(t, "config", "stale-days", "--project", "api"); out != "7\n" {
		t.Fatalf("global must be untouched by failed sets: got %q", out)
	}
}

func TestStatusResolvesProjectFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	project := filepath.Join(dir, "web")
	os.MkdirAll(filepath.Join(project, "src", "deep"), 0775)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "`+project+`"}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	now = func() time.Time { return t0 }
	run(t, "list", "--json") // import at t0
	now = func() time.Time { return t0.Add(3 * 24 * time.Hour) }
	t.Cleanup(func() { now = time.Now; statusJSON = false })

	if out := run(t, "status", "web"); out != "Quiet for 3 days\n" {
		t.Fatalf("by name: got %q", out)
	}
	t.Chdir(filepath.Join(project, "src", "deep"))
	if out := run(t, "status"); out != "Quiet for 3 days\n" {
		t.Fatalf("from subfolder: got %q", out)
	}
	want := `{
  "name": "web",
  "path": "` + project + `",
  "last_activity": "2026-10-01T09:00:00Z",
  "quiet_days": 3,
  "quiet_seconds": 259200,
  "note": "Quiet for 3 days",
  "short_note": "Quiet for 3 days",
  "stale": false
}
`
	if out := run(t, "status", "--json"); out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	t.Chdir(dir)
	rootCmd.SetArgs([]string{"status"})
	if rootCmd.Execute() == nil {
		t.Fatal("outside any project with no name should fail")
	}
}
