package cmd

import (
	"strings"
	"testing"
)

// Packages and installers check `proz --version` after installing, and
// release builds stamp the version with -ldflags "-X ...cmd.version=1.2.3".
func TestVersionFlagPrintsTheStampedVersion(t *testing.T) {
	configDir(t)
	old := rootCmd.Version
	t.Cleanup(func() { rootCmd.Version = old })
	rootCmd.Version = resolveVersion("1.2.3")

	out := run(t, "--version")

	if !strings.Contains(out, "1.2.3") {
		t.Fatalf("--version printed %q, want it to name 1.2.3", out)
	}
}

func TestUnstampedBuildsStillHaveAVersion(t *testing.T) {
	if v := resolveVersion(""); v == "" {
		t.Fatal("a build without -ldflags must still report a version")
	}
}
