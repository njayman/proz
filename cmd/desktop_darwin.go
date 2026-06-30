//go:build darwin

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func parseDesktopFiles() []desktopApp {
	dirs := appDirs()
	seen := make(map[string]bool)
	var apps []desktopApp

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}
			app := parseAppBundle(filepath.Join(dir, entry.Name()))
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

func appDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		"/Applications",
		"/Applications/Utilities",
		"/System/Applications",
		"/System/Applications/Utilities",
	}
	if home != "" {
		dirs = append([]string{filepath.Join(home, "Applications")}, dirs...)
	}
	return dirs
}

func parseAppBundle(path string) *desktopApp {
	plist := filepath.Join(path, "Contents", "Info.plist")
	if _, err := os.Stat(plist); err != nil {
		return nil
	}

	name, err := plistBuddy(plist, "CFBundleName")
	if err != nil {
		return nil
	}
	execName, err := plistBuddy(plist, "CFBundleExecutable")
	if err != nil {
		return nil
	}

	execPath := filepath.Join(path, "Contents", "MacOS", execName)
	if _, err := os.Stat(execPath); err != nil {
		return nil
	}

	return &desktopApp{Name: name, Exec: execPath}
}

func plistBuddy(plist, key string) (string, error) {
	cmd := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :"+key, plist)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
