package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNoTerminal means no terminal emulator could be found to open a terminal
// program in.
var ErrNoTerminal = errors.New("set the terminal setting with: proz config terminal <command>")

// terminalPrograms run in a terminal; anything else is treated as a GUI program.
// ponytail: a fixed list; #68 adds .desktop Terminal=true and per-program settings.
var terminalPrograms = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "hx": true, "helix": true, "nano": true,
	"micro": true, "kak": true, "lazygit": true, "tig": true,
	"claude": true, "codex": true, "gemini": true, "opencode": true, "aider": true,
}

func needsTerminal(program string) bool {
	return terminalPrograms[filepath.Base(program)]
}

// knownTerminals is the probe order, and how each runs a program in folder
// DIR, from the terminal research's table. Any other terminal gets -e.
var knownTerminals = []struct {
	name string
	run  []string
}{
	{"foot", []string{"-D", "DIR"}},
	{"kitty", []string{"-d", "DIR"}},
	{"wezterm", []string{"start", "--cwd", "DIR", "--"}},
	{"alacritty", []string{"--working-directory", "DIR", "-e"}},
	{"ghostty", []string{"--working-directory=DIR", "-e"}},
	{"ptyxis", []string{"--"}},
	{"kgx", []string{"-e"}},
	{"gnome-terminal", []string{"--working-directory=DIR", "--"}},
	{"konsole", []string{"--workdir", "DIR", "-e"}},
	{"xfce4-terminal", []string{"--working-directory=DIR", "-x"}},
	{"xterm", []string{"-e"}},
}

// runForm is the command that runs a program in terminal t, started in dir.
// t may carry its own flags: the terminal's own form is appended. With
// exact, a setting of several words is used as written instead.
func runForm(t, dir string, exact bool) []string {
	f := strings.Fields(t)
	if exact && len(f) > 1 {
		return f
	}
	name := filepath.Base(f[0])
	if name == "x-terminal-emulator" {
		// a Debian alternative: use the real terminal's form when it is a known one
		if p, err := exec.LookPath(f[0]); err == nil {
			if real, err := filepath.EvalSymlinks(p); err == nil {
				name = filepath.Base(real)
			}
		}
	}
	if name == "xdg-terminal-exec" {
		// no --dir: hand-written wrappers pass it on as the program; the working folder covers it
		return f
	}
	run := []string{"-e"}
	for _, k := range knownTerminals {
		if k.name == name {
			run = k.run
		}
	}
	for _, a := range run {
		f = append(f, strings.ReplaceAll(a, "DIR", dir))
	}
	return f
}

// runInTerminal finds the user's terminal emulator and returns the command
// that runs a program in it, started in dir. Linux order: the setting,
// xdg-terminal-exec, $TERMINAL, KDE's default, Sway's $term,
// x-terminal-emulator, then known terminals. A step naming a program that is
// not on PATH is skipped.
func (s *Store) runInTerminal(dir string) ([]string, error) {
	set, err := s.Terminal()
	if err != nil {
		return nil, err
	}
	steps := []func() string{
		func() string { return set },
		func() string { return "xdg-terminal-exec" },
		func() string { return os.Getenv("TERMINAL") },
		kdeTerminal,
		swayTerminal,
		func() string { return "x-terminal-emulator" },
	}
	for _, k := range knownTerminals {
		steps = append(steps, func() string { return k.name })
	}
	for i, step := range steps {
		t := step()
		if f := strings.Fields(t); len(f) > 0 {
			if _, err := exec.LookPath(f[0]); err == nil {
				return runForm(t, dir, i == 0), nil
			}
		}
	}
	return nil, ErrNoTerminal
}

// kdeTerminal is KDE's default terminal from kdeglobals, as KIO reads it:
// TerminalService (a .desktop id) before TerminalApplication (a command).
// ponytail: a service id is taken as its last name part
// (org.kde.konsole.desktop is konsole), not looked up.
func kdeTerminal() string {
	if !strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "KDE") {
		return ""
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, "kdeglobals"))
	if err != nil {
		return ""
	}
	var service, app string
	group := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			group = line
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || group != "[General]" {
			continue
		}
		switch strings.TrimSpace(k) {
		case "TerminalService":
			id := strings.TrimSuffix(strings.TrimSpace(v), ".desktop")
			service = id[strings.LastIndex(id, ".")+1:]
		case "TerminalApplication":
			app = strings.TrimSpace(v)
		}
	}
	if service != "" {
		return service
	}
	return app
}

// swayTerminal is $term from the running Sway's config, when in Sway.
func swayTerminal() string {
	if os.Getenv("SWAYSOCK") == "" {
		return ""
	}
	out, err := exec.Command("swaymsg", "-t", "get_config").Output()
	if err != nil {
		return ""
	}
	var c struct{ Config string }
	if json.Unmarshal(out, &c) != nil {
		return ""
	}
	for _, line := range strings.Split(c.Config, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "set $term "); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Terminal is the terminal setting: a command, empty to detect.
func (s *Store) Terminal() (string, error) {
	var t string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = 'terminal'`, DefaultWorkspace).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return t, err
}

// SetTerminal sets the terminal command; empty goes back to detecting it.
func (s *Store) SetTerminal(t string) error {
	if strings.TrimSpace(t) == "" {
		_, err := s.db.Exec(`DELETE FROM settings WHERE workspace_id = ? AND key = 'terminal'`, DefaultWorkspace)
		return err
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, 'terminal', ?)`, DefaultWorkspace, t)
	return err
}
