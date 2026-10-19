package core

import (
	"database/sql"
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

// knownTerminals is the probe order, and how each runs a program; any other
// terminal gets the common -e form.
var knownTerminals = []struct {
	name string
	run  []string
}{
	{"foot", nil}, {"kitty", nil}, {"wezterm", []string{"start", "--"}},
	{"alacritty", []string{"-e"}}, {"ghostty", []string{"-e"}},
	{"gnome-terminal", []string{"--"}}, {"konsole", []string{"-e"}},
	{"xfce4-terminal", []string{"-x"}}, {"xterm", []string{"-e"}},
}

// runForm is the command that runs a program in terminal t, which may carry
// its own flags: the terminal's own form is appended. With exact, a setting
// of several words is used as written instead.
func runForm(t string, exact bool) []string {
	f := strings.Fields(t)
	if exact && len(f) > 1 {
		return f
	}
	for _, k := range knownTerminals {
		if k.name == filepath.Base(f[0]) {
			return append(f, k.run...)
		}
	}
	return append(f, "-e")
}

// runInTerminal finds the user's terminal emulator and returns the command
// that runs a program in it. ponytail: Linux order without the desktop steps
// (xdg-terminal-exec, KDE, Sway, x-terminal-emulator); #67 adds them.
func (s *Store) runInTerminal() ([]string, error) {
	set, err := s.Terminal()
	if err != nil {
		return nil, err
	}
	for i, t := range []string{set, os.Getenv("TERMINAL")} {
		if f := strings.Fields(t); len(f) > 0 {
			if _, err := exec.LookPath(f[0]); err == nil {
				return runForm(t, i == 0), nil
			}
		}
	}
	for _, k := range knownTerminals {
		if _, err := exec.LookPath(k.name); err == nil {
			return runForm(k.name, false), nil
		}
	}
	return nil, ErrNoTerminal
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
