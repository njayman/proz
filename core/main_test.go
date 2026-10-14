package core

import (
	"os"
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
