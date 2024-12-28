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

func GetConfigFilePath() string {
	return filepath.Join(GetConfigFolder(), CONFIG_FILE_NAME)
}

func EnsureConfigFolderExists() {
	configPath := GetConfigFolder()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.MkdirAll(configPath, 0775); err != nil {
			fmt.Println("Error creating config directory. Exiting...")
			os.Exit(1)
		}
	}
}
