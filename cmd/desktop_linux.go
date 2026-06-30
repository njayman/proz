//go:build linux

package cmd

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func parseDesktopFiles() []desktopApp {
	dirs := desktopDataDirs()
	seen := make(map[string]bool)
	var apps []desktopApp

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
				continue
			}
			app := parseDesktopFile(filepath.Join(dir, entry.Name()))
			if app == nil || seen[app.Exec] {
				continue
			}
			seen[app.Exec] = true
			apps = append(apps, *app)
		}
	}

	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})

	return apps
}

func desktopDataDirs() []string {
	var dirs []string

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	dirs = append(dirs, filepath.Join(dataHome, "applications"))

	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	for _, d := range filepath.SplitList(dataDirs) {
		dirs = append(dirs, filepath.Join(d, "applications"))
	}

	return dirs
}

func parseDesktopFile(path string) *desktopApp {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var app desktopApp
	appType := ""
	tryExec := ""
	inDesktopEntry := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "[") {
			inDesktopEntry = line == "[Desktop Entry]"
			continue
		}
		if !inDesktopEntry || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "Type="):
			appType = strings.TrimPrefix(line, "Type=")
		case strings.HasPrefix(line, "Name="):
			app.Name = strings.TrimPrefix(line, "Name=")
		case strings.HasPrefix(line, "Exec="):
			exec := stripExecCodes(strings.TrimPrefix(line, "Exec="))
			app.Exec = strings.TrimSpace(exec)
		case strings.HasPrefix(line, "TryExec="):
			tryExec = strings.TrimPrefix(line, "TryExec=")
		}
	}

	if appType != "Application" || app.Name == "" || app.Exec == "" {
		return nil
	}
	if tryExec != "" {
		if _, err := exec.LookPath(tryExec); err != nil {
			return nil
		}
	}

	return &app
}
