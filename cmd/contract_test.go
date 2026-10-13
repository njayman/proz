package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

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

	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	rootCmd.SetArgs([]string{"list", "--json"})
	err := rootCmd.Execute()
	os.Stdout = stdout
	w.Close()
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

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
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
