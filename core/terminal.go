package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoTerminal means no terminal emulator could be found to open a terminal
// program in.
var ErrNoTerminal = errors.New("set the terminal setting with: proz config terminal <command>")

// terminalPrograms run in a terminal unless the user or the system says
// otherwise; anything else is treated as a GUI program.
var terminalPrograms = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "hx": true, "helix": true, "nano": true,
	"micro": true, "kak": true, "lazygit": true, "tig": true,
	"claude": true, "codex": true, "gemini": true, "opencode": true, "aider": true,
}

// NeedsTerminal is whether a program runs in a terminal, and whether that is
// the user's setting (Set) rather than proz's guess.
type NeedsTerminal struct {
	On  bool
	Set bool
}

// NeedsTerminal decides whether program runs in a terminal: the user's
// per-program setting, else a .desktop entry with Terminal=true, else the
// built-in list.
func (s *Store) NeedsTerminal(program string) (NeedsTerminal, error) {
	name := filepath.Base(program)
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE workspace_id = ? AND key = ?`, DefaultWorkspace, "needs_terminal."+name).Scan(&v)
	if err == nil {
		return NeedsTerminal{On: v == "on", Set: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return NeedsTerminal{}, err
	}
	return NeedsTerminal{On: desktopSaysTerminal(name) || terminalPrograms[name]}, nil
}

// SetNeedsTerminal sets whether program runs in a terminal; nil goes back
// to proz's guess.
func (s *Store) SetNeedsTerminal(program string, on *bool) error {
	key := "needs_terminal." + filepath.Base(program)
	if on == nil {
		_, err := s.db.Exec(`DELETE FROM settings WHERE workspace_id = ? AND key = ?`, DefaultWorkspace, key)
		return err
	}
	v := "off"
	if *on {
		v = "on"
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (workspace_id, key, value) VALUES (?, ?, ?)`, DefaultWorkspace, key, v)
	return err
}

// desktopSaysTerminal reports whether an installed .desktop entry runs
// program with Terminal=true.
func desktopSaysTerminal(program string) bool {
	for _, dir := range DesktopDirs() {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var execName string
			terminal, inEntry := false, false
			for _, line := range strings.Split(string(b), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "[") {
					inEntry = line == "[Desktop Entry]"
				} else if v, ok := strings.CutPrefix(line, "Exec="); ok && inEntry {
					if f := strings.Fields(v); len(f) > 0 {
						execName = filepath.Base(strings.Trim(f[0], `"`))
					}
				} else if inEntry && line == "Terminal=true" {
					terminal = true
				}
			}
			if terminal && execName == program {
				return true
			}
		}
	}
	return false
}

// DesktopDirs are the folders holding .desktop entries, most specific first.
func DesktopDirs() []string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	dirs := []string{filepath.Join(dataHome, "applications")}
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	for _, d := range filepath.SplitList(dataDirs) {
		dirs = append(dirs, filepath.Join(d, "applications"))
	}
	return dirs
}

// knownTerminals is the probe order, and how each runs a program in folder
// DIR, from the table in the terminal research (research/terminal-detection
// branch). Any other terminal gets -e. ponytail: one fixed probe order; the
// research suggests desktop-aware first (ptyxis, kgx on GNOME) if it matters.
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
			if target, err := filepath.EvalSymlinks(p); err == nil {
				name = filepath.Base(target)
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
// TerminalService (a .desktop id) when it is on PATH, else
// TerminalApplication (a command). ponytail: a service id is taken as its
// last name part, lowercased (org.kde.konsole.desktop is konsole), not
// looked up in its .desktop file.
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
		k, _, _ = strings.Cut(k, "[") // TerminalApplication[$e] is the same key
		switch strings.TrimSpace(k) {
		case "TerminalService":
			id := strings.TrimSuffix(strings.TrimSpace(v), ".desktop")
			service = strings.ToLower(id[strings.LastIndex(id, ".")+1:])
		case "TerminalApplication":
			app = strings.TrimSpace(v)
		}
	}
	if _, err := exec.LookPath(service); service != "" && err == nil {
		return service
	}
	return app
}

// swayTerminal is $term from the running Sway's config, when in Sway: the
// last set wins, and a value naming an earlier variable is expanded.
// ponytail: only the main config file; sets in included files are missed.
func swayTerminal() string {
	if os.Getenv("SWAYSOCK") == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "swaymsg", "-t", "get_config").Output()
	if err != nil {
		return ""
	}
	var c struct{ Config string }
	if json.Unmarshal(out, &c) != nil {
		return ""
	}
	vars := map[string]string{}
	for _, line := range strings.Split(c.Config, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "set" || !strings.HasPrefix(f[1], "$") {
			continue
		}
		v := strings.Trim(strings.Join(f[2:], " "), `"'`)
		if e, ok := vars[v]; ok {
			v = e
		}
		vars[f[1]] = v
	}
	return vars["$term"]
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
