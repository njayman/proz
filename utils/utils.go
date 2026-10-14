package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetConfigFolder() string {
	configDir, err := os.UserConfigDir()

	if err != nil {
		fmt.Println("Error locating user config directory. Exiting...")
		os.Exit(1)
	}

	return filepath.Join(configDir, CONFIG_FOLDER_NAME)
}
