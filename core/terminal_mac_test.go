package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// onMac makes core behave as on macOS, with a temp home and these apps
// installed in ~/Applications.
func onMac(t *testing.T, apps ...string) {
	t.Helper()
	fakePath(t, "osascript", "open")
	was := goos
	goos = "darwin"
	t.Cleanup(func() { goos = was })
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, a := range apps {
		if err := os.MkdirAll(filepath.Join(home, "Applications", a+".app"), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func app(name, bin string) string {
	return filepath.Join(os.Getenv("HOME"), "Applications", name+".app", "Contents", "MacOS", bin)
}

func TestMacUsesTheFirstPreferredApp(t *testing.T) {
	onMac(t, "Alacritty", "kitty", "WezTerm")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "nvim", false), []string{app("WezTerm", "wezterm"), "start", "--cwd", p.Path, "--", "nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMacAppForms(t *testing.T) {
	cases := map[string]func(dir string) []string{
		"iTerm": func(dir string) []string {
			return []string{"osascript", "-e", iTermScript, "cd '" + dir + "' && exec 'nvim' '" + dir + "'"}
		},
		"kitty": func(dir string) []string { return []string{app("kitty", "kitty"), "-d", dir, "nvim", dir} },
		"Ghostty": func(dir string) []string {
			return []string{"open", "-na", filepath.Join(os.Getenv("HOME"), "Applications", "Ghostty.app"), "--args", "--working-directory=" + dir, "-e", "nvim", dir}
		},
		"Alacritty": func(dir string) []string {
			return []string{app("Alacritty", "alacritty"), "--working-directory", dir, "-e", "nvim", dir}
		},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			onMac(t, name)
			s := open(t, t.TempDir())
			p := project(t, s)
			if got := launchArgs(t, s, p, "nvim", false); !reflect.DeepEqual(got, want(p.Path)) {
				t.Fatalf("got %v, want %v", got, want(p.Path))
			}
		})
	}
}

func TestMacFallsBackToTerminalApp(t *testing.T) {
	onMac(t)
	s := open(t, t.TempDir())
	p := project(t, s)

	want := []string{"osascript", "-e", terminalAppScript, "cd '" + p.Path + "' && exec 'nvim' '" + p.Path + "'"}
	if got := launchArgs(t, s, p, "nvim", false); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	cmd, err := s.LaunchTerminal(p, false)
	if err != nil || !reflect.DeepEqual(cmd.Args, []string{"osascript", "-e", terminalAppScript, "cd '" + p.Path + "'"}) {
		t.Fatalf("terminal only: got %v, %v", cmd.Args, err)
	}
}

func TestMacQuotesTheFolderForTheShell(t *testing.T) {
	onMac(t)
	s := open(t, t.TempDir())
	dir := filepath.Join(t.TempDir(), "it's mine")
	os.Mkdir(dir, 0755)
	s.AddProject(Project{Name: "quoted", Path: dir})
	p, _ := s.ProjectNamed("quoted")

	got := launchArgs(t, s, p, "nvim", false)
	if want := `cd '` + filepath.Dir(dir) + `/it'\''s mine' && exec 'nvim' '` + filepath.Dir(dir) + `/it'\''s mine'`; got[3] != want {
		t.Fatalf("got %q, want %q", got[3], want)
	}
}

func TestMacSettingWins(t *testing.T) {
	onMac(t, "iTerm")
	fakeExec(t, "wezterm", "")
	s := open(t, t.TempDir())
	p := project(t, s)
	if err := s.SetTerminal("wezterm"); err != nil {
		t.Fatal(err)
	}

	if got := launchArgs(t, s, p, "nvim", false); got[0] != "wezterm" {
		t.Fatalf("got %v, want the setting", got)
	}
}
