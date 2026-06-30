//go:build windows

package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func parseDesktopFiles() []desktopApp {
	uninstallMap := readUninstallEntries()
	var apps []desktopApp
	seen := make(map[string]bool)

	roots := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`},
	}

	for _, r := range roots {
		k, err := registry.OpenKey(r.root, r.path, registry.READ)
		if err != nil {
			continue
		}
		subkeys, err := k.ReadSubKeyNames(0)
		k.Close()
		if err != nil {
			continue
		}

		for _, subkey := range subkeys {
			sk, err := registry.OpenKey(r.root, r.path+`\`+subkey, registry.READ)
			if err != nil {
				continue
			}
			execPath, _, err := sk.GetStringValue("")
			sk.Close()
			if err != nil || execPath == "" {
				continue
			}

			if seen[execPath] {
				continue
			}
			seen[execPath] = true

			name := deriveName(subkey, execPath, uninstallMap)
			apps = append(apps, desktopApp{Name: name, Exec: execPath})
		}
	}

	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})

	return apps
}

func readUninstallEntries() map[string]string {
	result := make(map[string]string)
	roots := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}
	uninstallPath := `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`

	for _, root := range roots {
		k, err := registry.OpenKey(root, uninstallPath, registry.READ)
		if err != nil {
			continue
		}
		subkeys, err := k.ReadSubKeyNames(0)
		k.Close()
		if err != nil {
			continue
		}

		for _, subkey := range subkeys {
			sk, err := registry.OpenKey(root, uninstallPath+`\`+subkey, registry.READ)
			if err != nil {
				continue
			}

			displayName, _, err := sk.GetStringValue("DisplayName")
			if err != nil || displayName == "" {
				sk.Close()
				continue
			}

			key := ""
			if iconPath, _, err := sk.GetStringValue("DisplayIcon"); err == nil && iconPath != "" {
				key = strings.Split(iconPath, ",")[0]
			} else if installLoc, _, err := sk.GetStringValue("InstallLocation"); err == nil && installLoc != "" {
				key = installLoc
			}
			sk.Close()

			if key != "" {
				result[strings.ToLower(key)] = displayName
			}
		}
	}

	return result
}

func deriveName(subkey, execPath string, uninstallMap map[string]string) string {
	if name, ok := uninstallMap[strings.ToLower(execPath)]; ok {
		return name
	}
	if name, ok := uninstallMap[strings.ToLower(filepath.Dir(execPath))]; ok {
		return name
	}

	name := strings.TrimSuffix(subkey, filepath.Ext(subkey))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}

	path := strings.Join(words, " ")
	if _, err := os.Stat(execPath); err == nil {
		return path
	}
	return path
}
