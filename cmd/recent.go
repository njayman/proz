package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/njayman/proz/utils"
)

const maxRecent = 4

func recentFilePath() string {
	return filepath.Join(utils.GetConfigFolder(), "recent.json")
}

func loadRecentExecs() []string {
	data, err := os.ReadFile(recentFilePath())
	if err != nil {
		return nil
	}
	var execs []string
	if err := json.Unmarshal(data, &execs); err != nil {
		return nil
	}
	if len(execs) > maxRecent {
		execs = execs[:maxRecent]
	}
	return execs
}

func saveRecentExecs(execs []string) error {
	data, err := json.MarshalIndent(execs, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(recentFilePath()), 0775)
	return os.WriteFile(recentFilePath(), data, 0644)
}

func pushRecentExec(exec string) {
	execs := loadRecentExecs()
	filtered := make([]string, 0, maxRecent)
	for _, e := range execs {
		if e != exec {
			filtered = append(filtered, e)
		}
	}
	result := append([]string{exec}, filtered...)
	if len(result) > maxRecent {
		result = result[:maxRecent]
	}
	saveRecentExecs(result)
}
