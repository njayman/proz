package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func listPathBinaries() []string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}
	dirs := filepath.SplitList(pathEnv)
	binaryMap := make(map[string]bool)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.Mode()&0111 != 0 {
				binaryMap[entry.Name()] = true
			}
		}
	}
	binaries := make([]string, 0, len(binaryMap))
	for bin := range binaryMap {
		binaries = append(binaries, bin)
	}
	sort.Strings(binaries)
	return binaries
}

func hasAnyTag(projectTags, filterTags []string) bool {
	for _, ft := range filterTags {
		ft = strings.TrimSpace(ft)
		for _, pt := range projectTags {
			if strings.EqualFold(pt, ft) {
				return true
			}
		}
	}
	return false
}
