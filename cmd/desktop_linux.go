//go:build linux

package cmd

import (
	"sort"
	"strings"

	"github.com/njayman/proz/core"
)

func parseDesktopFiles() []desktopApp {
	var apps []desktopApp
	for _, e := range core.DesktopEntries() {
		apps = append(apps, desktopApp{Name: e.Name, Exec: e.Exec})
	}
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps
}
