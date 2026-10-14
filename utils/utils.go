package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetConfigFolder() string {
	configDir, err := os.UserConfigDir()

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error locating user config directory. Exiting...") // stderr: under 'proz mcp' stdout is the protocol
		os.Exit(1)
	}

	return filepath.Join(configDir, CONFIG_FOLDER_NAME)
}
