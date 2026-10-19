package core

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DesktopEntry is an installed application from a freedesktop .desktop file.
type DesktopEntry struct {
	Name     string
	Exec     string // the command line, field codes (%F, %U) removed
	Terminal bool   // Terminal=true: it runs in a terminal
}

// Program is the name of the program the entry runs, past any leading
// env VAR=value.
func (e DesktopEntry) Program() string {
	f := strings.Fields(e.Exec)
	for len(f) > 0 && (f[0] == "env" || strings.Contains(f[0], "=")) {
		f = f[1:]
	}
	if len(f) == 0 {
		return ""
	}
	return filepath.Base(strings.Trim(f[0], `"`))
}

// DesktopEntries are the installed applications, one per command line, the
// most specific folder first. Entries whose TryExec is not on PATH are left out.
func DesktopEntries() []DesktopEntry {
	seen := map[string]bool{}
	var out []DesktopEntry
	for _, dir := range DesktopDirs() {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".desktop") {
				continue
			}
			e := parseDesktopEntry(filepath.Join(dir, f.Name()))
			if e == nil || seen[e.Exec] {
				continue
			}
			seen[e.Exec] = true
			out = append(out, *e)
		}
	}
	return out
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

func parseDesktopEntry(path string) *DesktopEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var e DesktopEntry
	appType, tryExec := "", ""
	inEntry := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Type="):
			appType = strings.TrimPrefix(line, "Type=")
		case strings.HasPrefix(line, "Name="):
			e.Name = strings.TrimPrefix(line, "Name=")
		case strings.HasPrefix(line, "Exec="):
			e.Exec = stripExecCodes(strings.TrimPrefix(line, "Exec="))
		case strings.HasPrefix(line, "TryExec="):
			tryExec = strings.TrimPrefix(line, "TryExec=")
		case line == "Terminal=true":
			e.Terminal = true
		}
	}
	if appType != "Application" || e.Name == "" || e.Exec == "" {
		return nil
	}
	if tryExec != "" {
		if _, err := exec.LookPath(tryExec); err != nil {
			return nil
		}
	}
	return &e
}

// stripExecCodes removes field codes such as %F from an Exec line.
func stripExecCodes(exec string) string {
	var b strings.Builder
	for i := 0; i < len(exec); i++ {
		if exec[i] == '%' && i+1 < len(exec) {
			i++
			continue
		}
		b.WriteByte(exec[i])
	}
	return strings.TrimSpace(b.String())
}
