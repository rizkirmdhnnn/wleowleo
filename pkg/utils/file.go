package utils

import (
	"os"
	"path/filepath"
)

func ClearFolder(folderPath string) error {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		entryPath := filepath.Join(folderPath, entry.Name())

		// Remove file or directory (including its contents if it's a directory)
		err = os.RemoveAll(entryPath)
		if err != nil {
			return err
		}
	}

	return nil
}
