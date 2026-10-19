package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain keeps tests away from the real ~/.claude session logs.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "proz-claude")
	if err != nil {
		panic(err)
	}
	os.Setenv("CLAUDE_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// configDir points proz's config folder (os.UserConfigDir) at a new temp
// folder and returns it.
func configDir(t *testing.T) string {
	dir := t.TempDir()
	useConfigDir(t, dir)
	return dir
}

// useConfigDir makes os.UserConfigDir return dir, on every system.
func useConfigDir(t *testing.T, dir string) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", dir)
	case "darwin": // $HOME/Library/Application Support
		home := t.TempDir()
		os.Mkdir(filepath.Join(home, "Library"), 0o755)
		if err := os.Symlink(dir, filepath.Join(home, "Library", "Application Support")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
	default:
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
}
