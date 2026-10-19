package core

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// fakePath makes core behave as on Linux, with PATH holding only fake
// executables with these names. Fake scripts need a Unix system to run.
func fakePath(t *testing.T, names ...string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake terminals are shell scripts")
	}
	was := goos
	goos = "linux"
	t.Cleanup(func() { goos = was })
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("TERMINAL", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("SWAYSOCK", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
}

// desktopEntry installs a .desktop entry in the user's applications folder.
func desktopEntry(t *testing.T, name, body string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "applications")
	os.MkdirAll(dir, 0755)
	if err := os.WriteFile(filepath.Join(dir, name+".desktop"), []byte("[Desktop Entry]\nType=Application\nName="+name+"\n"+body+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// fakeExec writes an executable script into the first PATH folder.
func fakeExec(t *testing.T, name, script string) {
	t.Helper()
	dir := filepath.SplitList(os.Getenv("PATH"))[0]
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0755); err != nil {
		t.Fatal(err)
	}
}

// kdeSession makes this a KDE session whose kdeglobals has these lines.
func kdeSession(t *testing.T, lines string) {
	t.Helper()
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	if err := os.WriteFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "kdeglobals"), []byte("[Icons]\nTheme=breeze\n\n[General]\n"+lines+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// swaySession makes this a Sway session whose loaded config sets $term.
func swaySession(t *testing.T, term string) {
	t.Helper()
	t.Setenv("SWAYSOCK", "/run/sway.sock")
	fakeExec(t, "swaymsg", `[ "$1 $2" = "-t get_config" ] || exit 1
printf '%s' '{"config": "set $mod Mod4\nset $term `+term+`\nbindsym $mod+Return exec $term\n"}'
`)
}

// project adds one project in a temp folder and returns it.
func project(t *testing.T, s *Store) Project {
	t.Helper()
	if err := s.AddProject(Project{Name: "web", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	ps, _ := s.Projects()
	return ps[0]
}

func launchArgs(t *testing.T, s *Store, p Project, program string, attached bool) []string {
	t.Helper()
	cmd, err := s.Launch(p, program, attached)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != p.Path {
		t.Fatalf("working folder %q, want the project's %q", cmd.Dir, p.Path)
	}
	return cmd.Args
}

func TestTerminalProgramOpensInTerminalWithoutOne(t *testing.T) {
	fakePath(t, "myterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)

	got := launchArgs(t, s, p, "nvim", false)
	if want := []string{"myterm", "-e", "nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTerminalProgramRunsInPlaceWithATerminal(t *testing.T) {
	fakePath(t, "myterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "nvim", true), []string{"nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestGUIProgramIsNeverWrapped(t *testing.T) {
	fakePath(t, "myterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "code", false), []string{"code", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTerminalSettingBeatsTERMINAL(t *testing.T) {
	fakePath(t, "myterm", "wezterm")
	t.Setenv("TERMINAL", "myterm")
	s := open(t, t.TempDir())
	p := project(t, s)
	if err := s.SetTerminal("wezterm"); err != nil {
		t.Fatal(err)
	}

	if got, want := launchArgs(t, s, p, "claude", false), []string{"wezterm", "start", "--cwd", p.Path, "--", "claude", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// several words are used as written
	if err := s.SetTerminal("myterm --hold -x"); err != nil {
		t.Fatal(err)
	}
	if got, want := launchArgs(t, s, p, "claude", false), []string{"myterm", "--hold", "-x", "claude", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// empty goes back to detecting
	if err := s.SetTerminal(""); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Terminal(); err != nil || got != "" {
		t.Fatalf("cleared setting: got %q, %v", got, err)
	}
	if got := launchArgs(t, s, p, "claude", false); got[0] != "myterm" || got[1] != "-e" {
		t.Fatalf("got %v, want $TERMINAL with -e", got)
	}
}

func TestMissingTERMINALFallsBackToAKnownTerminal(t *testing.T) {
	fakePath(t, "xterm")
	t.Setenv("TERMINAL", "gone")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "lazygit", false), []string{"xterm", "-e", "lazygit", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNoTerminalNamesTheSettingAndStillCountsAsActivity(t *testing.T) {
	fakePath(t)
	c := &clock{t0}
	s := openAt(t, c)
	p := project(t, s)

	c.t = t0.Add(5 * day)
	_, err := s.Launch(p, "nvim", false)
	if err == nil || !strings.Contains(err.Error(), "proz config terminal") {
		t.Fatalf("want an error naming the setting, got %v", err)
	}
	st, err := s.Status(p.ID)
	if err != nil || !st.LastActivity.Equal(t0.Add(5*day)) {
		t.Fatalf("want the launch recorded, got %v, %v", st.LastActivity, err)
	}
}

func TestTERMINALWithFlagsStillGetsARunForm(t *testing.T) {
	fakePath(t, "alacritty")
	t.Setenv("TERMINAL", "alacritty --class work")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "nvim", false), []string{"alacritty", "--class", "work", "--working-directory", p.Path, "-e", "nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLinuxOrderFirstPresentStepWins(t *testing.T) {
	steps := []string{"setting", "xdg-terminal-exec", "$TERMINAL", "kde", "sway", "x-terminal-emulator", "probe"}
	want := []string{"setterm", "xdg-terminal-exec", "envterm", "kdeterm", "swayterm", "x-terminal-emulator", "xterm"}
	for i, step := range steps {
		t.Run(step, func(t *testing.T) {
			fakePath(t, "setterm", "envterm", "kdeterm", "swayterm", "x-terminal-emulator", "xterm")
			s := open(t, t.TempDir())
			p := project(t, s)
			// every step from this one down is present
			for _, later := range steps[i:] {
				switch later {
				case "setting":
					if err := s.SetTerminal("setterm"); err != nil {
						t.Fatal(err)
					}
				case "xdg-terminal-exec":
					fakeExec(t, "xdg-terminal-exec", "")
				case "$TERMINAL":
					t.Setenv("TERMINAL", "envterm")
				case "kde":
					kdeSession(t, "TerminalApplication=kdeterm")
				case "sway":
					swaySession(t, "swayterm")
				}
			}
			if i > slices.Index(steps, "x-terminal-emulator") {
				os.Remove(filepath.Join(os.Getenv("PATH"), "x-terminal-emulator"))
			}
			if got := launchArgs(t, s, p, "nvim", false); got[0] != want[i] {
				t.Fatalf("got %v, want %s first", got, want[i])
			}
		})
	}
}

func TestStepsNamingMissingProgramsAreSkipped(t *testing.T) {
	fakePath(t, "x-terminal-emulator")
	t.Setenv("TERMINAL", "gone1")
	kdeSession(t, "TerminalApplication=gone2")
	swaySession(t, "gone3")
	s := open(t, t.TempDir())
	p := project(t, s)
	if err := s.SetTerminal("gone0"); err != nil {
		t.Fatal(err)
	}

	if got, want := launchArgs(t, s, p, "nvim", false), []string{"x-terminal-emulator", "-e", "nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKDETerminalServiceNamesTheTerminal(t *testing.T) {
	fakePath(t, "konsole", "xterm")
	kdeSession(t, "TerminalApplication=xterm\nTerminalService=org.kde.konsole.desktop")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "nvim", false), []string{"konsole", "--workdir", p.Path, "-e", "nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKDESettingIgnoredOutsideKDE(t *testing.T) {
	fakePath(t, "kdeterm", "xterm")
	kdeSession(t, "TerminalApplication=kdeterm")
	t.Setenv("XDG_CURRENT_DESKTOP", "sway")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got := launchArgs(t, s, p, "nvim", false); got[0] != "xterm" {
		t.Fatalf("got %v, want xterm", got)
	}
}

func TestEachKnownTerminalRunsInTheProjectFolder(t *testing.T) {
	for term, form := range map[string][]string{
		"xdg-terminal-exec": {}, // no --dir: wrappers pass it on as the program; the working folder covers it
		"wezterm":           {"start", "--cwd", "DIR", "--"},
		"foot":              {"-D", "DIR"},
		"kitty":             {"-d", "DIR"},
		"alacritty":         {"--working-directory", "DIR", "-e"},
		"gnome-terminal":    {"--working-directory=DIR", "--"},
		"konsole":           {"--workdir", "DIR", "-e"},
		"ghostty":           {"--working-directory=DIR", "-e"},
		"xfce4-terminal":    {"--working-directory=DIR", "-x"},
		"ptyxis":            {"--"},
		"kgx":               {"-e"},
		"xterm":             {"-e"},
		"someterm":          {"-e"},
	} {
		t.Run(term, func(t *testing.T) {
			fakePath(t, term)
			t.Setenv("TERMINAL", term)
			s := open(t, t.TempDir())
			p := project(t, s)
			want := []string{term}
			for _, a := range form {
				want = append(want, strings.ReplaceAll(a, "DIR", p.Path))
			}
			want = append(want, "nvim", p.Path)
			if got := launchArgs(t, s, p, "nvim", false); !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestXTerminalEmulatorUsesTheRealTerminalsForm(t *testing.T) {
	fakePath(t, "wezterm")
	dir := os.Getenv("PATH")
	if err := os.Symlink(filepath.Join(dir, "wezterm"), filepath.Join(dir, "x-terminal-emulator")); err != nil {
		t.Fatal(err)
	}
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "nvim", false), []string{"x-terminal-emulator", "start", "--cwd", p.Path, "--", "nvim", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKDEServiceIsLowercasedAndFallsBackToTheApplication(t *testing.T) {
	fakePath(t, "ptyxis", "kdeterm", "xterm")
	s := open(t, t.TempDir())
	p := project(t, s)

	kdeSession(t, "TerminalService=org.gnome.Ptyxis.desktop")
	if got := launchArgs(t, s, p, "nvim", false); got[0] != "ptyxis" {
		t.Fatalf("got %v, want ptyxis", got)
	}
	kdeSession(t, "TerminalService=gone.desktop\nTerminalApplication[$e]=kdeterm")
	if got := launchArgs(t, s, p, "nvim", false); got[0] != "kdeterm" {
		t.Fatalf("got %v, want kdeterm", got)
	}
}

func TestSwayTermLastSetWinsAndVariablesExpand(t *testing.T) {
	for config, want := range map[string]string{
		`set $term foot\nset $term swayterm`:        "swayterm",
		`set $myterm swayterm\nset\t$term\t$myterm`: "swayterm",
		`set $term \"swayterm\"`:                    "swayterm",
	} {
		t.Run(want+" from "+config, func(t *testing.T) {
			fakePath(t, "swayterm", "xterm")
			t.Setenv("SWAYSOCK", "/run/sway.sock")
			fakeExec(t, "swaymsg", `printf '%s' '{"config": "`+config+`"}'`)
			s := open(t, t.TempDir())
			p := project(t, s)
			if got := launchArgs(t, s, p, "nvim", false); got[0] != want {
				t.Fatalf("got %v, want %s", got, want)
			}
		})
	}
}

func TestDesktopEntrySaysTerminalTrue(t *testing.T) {
	fakePath(t, "xterm")
	desktopEntry(t, "btop", "Exec=/usr/bin/btop %F\nTerminal=true")
	desktopEntry(t, "gimp", "Exec=gimp-2.10 %U\nTerminal=false")
	desktopEntry(t, "yazi", "# Terminal=false\nExec=env TERM=xterm-256color yazi\nTerminal=true")
	s := open(t, t.TempDir())
	p := project(t, s)

	if got, want := launchArgs(t, s, p, "btop", false), []string{"xterm", "-e", "btop", p.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := launchArgs(t, s, p, "yazi", false); got[0] != "xterm" {
		t.Fatalf("env prefix: got %v, want it wrapped", got)
	}
	if got := launchArgs(t, s, p, "gimp-2.10", false); got[0] != "gimp-2.10" {
		t.Fatalf("Terminal=false: got %v, want it unwrapped", got)
	}
}

func TestPerProgramSettingBeatsEverything(t *testing.T) {
	fakePath(t, "xterm")
	desktopEntry(t, "btop", "Exec=btop\nTerminal=true")
	s := open(t, t.TempDir())
	p := project(t, s)
	for program, on := range map[string]bool{"nvim": false, "btop": false, "code": true} {
		if err := s.SetNeedsTerminal(program, &on); err != nil {
			t.Fatal(err)
		}
	}

	for program, wrapped := range map[string]bool{"nvim": false, "btop": false, "code": true} {
		if got := launchArgs(t, s, p, program, false); (got[0] == "xterm") != wrapped {
			t.Errorf("%s: got %v, want wrapped %v", program, got, wrapped)
		}
	}

	// removing the override goes back to the guess
	if err := s.SetNeedsTerminal("nvim", nil); err != nil {
		t.Fatal(err)
	}
	if got, err := s.NeedsTerminal("nvim"); err != nil || !got.On || got.Set {
		t.Fatalf("cleared: got %+v, %v", got, err)
	}
}

func TestTerminalOnlyOpensTheTerminalInTheProjectFolder(t *testing.T) {
	for term, want := range map[string][]string{
		"wezterm":           {"wezterm", "start", "--cwd", "DIR"},
		"alacritty":         {"alacritty", "--working-directory", "DIR"},
		"gnome-terminal":    {"gnome-terminal", "--working-directory=DIR"},
		"xterm":             {"xterm"},
		"xdg-terminal-exec": {"xdg-terminal-exec"},
	} {
		t.Run(term, func(t *testing.T) {
			fakePath(t, term)
			t.Setenv("TERMINAL", term)
			c := &clock{t0}
			s := openAt(t, c)
			p := project(t, s)
			c.t = t0.Add(5 * day)

			cmd, err := s.LaunchTerminal(p, false)
			if err != nil {
				t.Fatal(err)
			}
			for i := range want {
				want[i] = strings.ReplaceAll(want[i], "DIR", p.Path)
			}
			if !reflect.DeepEqual(cmd.Args, want) || cmd.Dir != p.Path {
				t.Fatalf("got %v in %q, want %v in %q", cmd.Args, cmd.Dir, want, p.Path)
			}
			if st, _ := s.Status(p.ID); !st.LastActivity.Equal(t0.Add(5 * day)) {
				t.Fatalf("want the launch recorded, got %v", st.LastActivity)
			}
		})
	}
}

func TestTerminalOnlyWithATerminalStartsTheShell(t *testing.T) {
	fakePath(t, "xterm")
	t.Setenv("SHELL", "/bin/zsh")
	s := open(t, t.TempDir())
	p := project(t, s)

	cmd, err := s.LaunchTerminal(p, true)
	if err != nil || !reflect.DeepEqual(cmd.Args, []string{"/bin/zsh"}) || cmd.Dir != p.Path {
		t.Fatalf("got %v in %q, %v", cmd.Args, cmd.Dir, err)
	}
}

func TestTerminalOnlyWithNoTerminalNamesTheSetting(t *testing.T) {
	fakePath(t)
	s := open(t, t.TempDir())
	p := project(t, s)

	if _, err := s.LaunchTerminal(p, false); err == nil || !strings.Contains(err.Error(), "proz config terminal") {
		t.Fatalf("want an error naming the setting, got %v", err)
	}
}
