package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndRemoveWriteTheLoginEntry(t *testing.T) {
	home := t.TempDir()
	for goos, want := range map[string]struct{ path, contains string }{
		"linux":  {filepath.Join(home, ".config", "autostart", "proz.desktop"), "Exec=\"/opt/my proz/proz\" tray\n"},
		"darwin": {filepath.Join(home, "Library", "LaunchAgents", "dev.proz.tray.plist"), "<string>/opt/my proz/proz</string>\n\t\t<string>tray</string>"},
	} {
		e := entryFor(goos, home, filepath.Join(home, ".config"))
		if e.path != want.path {
			t.Fatalf("%s: path %s", goos, e.path)
		}
		if err := e.install("/opt/my proz/proz"); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(want.path)
		if err != nil || !strings.Contains(string(b), want.contains) {
			t.Fatalf("%s: got %q, %v", goos, b, err)
		}
		if !e.installed() {
			t.Fatalf("%s: installed should be true", goos)
		}
		if err := e.remove(); err != nil || e.installed() {
			t.Fatalf("%s: remove: %v", goos, err)
		}
		if err := e.remove(); err != nil {
			t.Fatalf("%s: removing twice is fine: %v", goos, err)
		}
	}
}

func TestPlistEscapesThePath(t *testing.T) {
	e := entryFor("darwin", t.TempDir(), "")
	e.install("/Apps/a&b<c>/proz")
	b, _ := os.ReadFile(e.path)
	if !strings.Contains(string(b), "<string>/Apps/a&amp;b&lt;c&gt;/proz</string>") {
		t.Fatalf("got %s", b)
	}
}

func TestDesktopExecEscapesSpecialCharacters(t *testing.T) {
	// quoted-argument escapes (\" \$ \\), then the string value doubles each
	// backslash, and % is a field code so it doubles too
	got := desktop(`/opt/a"b$c\d%e/proz`)
	want := `Exec="/opt/a\\"b\\$c\\\\d%%e/proz" tray` + "\n"
	if !strings.Contains(got, want) {
		t.Fatalf("got %q", got)
	}
}
