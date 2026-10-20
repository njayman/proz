package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// onWindows makes core behave as on Windows, with a console and a GUI test
// executable built on PATH.
func onWindows(t *testing.T) {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("needs the go tool to build test executables")
	}
	src := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(src, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	for name, flags := range map[string]string{"console.exe": "", "gui.exe": "-H=windowsgui"} {
		build := exec.Command(gobin, "build", "-ldflags="+flags, "-o", filepath.Join(bin, name), src)
		build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("building %s: %v\n%s", name, err, out)
		}
	}
	was := goos
	goos = "windows"
	t.Cleanup(func() { goos = was })
}

func TestWindowsConsoleProgramsAreDetected(t *testing.T) {
	onWindows(t)
	s := open(t, t.TempDir())
	for program, want := range map[string]bool{"console.exe": true, "gui.exe": false, "console": true, "missing.exe": false} {
		if got, err := s.NeedsTerminal(program); err != nil || got.On != want {
			t.Errorf("%s: got %+v, %v, want %v", program, got, err, want)
		}
	}

	off := false
	if err := s.SetNeedsTerminal("console.exe", &off); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.NeedsTerminal("console.exe"); got.On {
		t.Fatal("the per-program setting should win")
	}
}

func TestWindowsConsoleProgramRunsAsItselfWithNoSetting(t *testing.T) {
	onWindows(t)
	s := open(t, t.TempDir())
	p := project(t, s)

	// a new console, which Windows hands to the default terminal; the flag is
	// checked in the Windows-only test
	if got, want := launchArgs(t, s, p, "console.exe", false), []string{"console.exe", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWindowsTerminalSetting(t *testing.T) {
	onWindows(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "wt.exe"), nil, 0755); err != nil {
		t.Fatal(err)
	}
	s := open(t, t.TempDir())
	dir := filepath.Join(t.TempDir(), "x;y")
	os.Mkdir(dir, 0755)
	s.AddProject(Project{Name: "semi", Path: dir, Arguments: []string{"a;b"}})
	p, _ := s.ProjectNamed("semi")
	esc := strings.ReplaceAll(dir, ";", `\;`)
	if err := s.SetTerminal("wt.exe"); err != nil {
		t.Fatal(err)
	}

	if got, want := launchArgs(t, s, p, "console.exe", false), []string{"wt.exe", "new-tab", "-d", esc, "console.exe", `a\;b`, esc}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	cmd, err := s.LaunchTerminal(p, "", false)
	if err != nil || !reflect.DeepEqual(cmd.Args, []string{"wt.exe", "new-tab", "-d", esc}) {
		t.Fatalf("terminal only: got %v, %v", cmd.Args, err)
	}
}

func TestWindowsMissingTerminalSettingIsAnError(t *testing.T) {
	onWindows(t)
	s := open(t, t.TempDir())
	p := project(t, s)
	if err := s.SetTerminal("wt.exe"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Launch(p, "", "console.exe", false); err == nil || !strings.Contains(err.Error(), "proz config terminal") {
		t.Fatalf("want an error naming the setting, got %v", err)
	}
}
