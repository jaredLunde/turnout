package fs

import (
	"os"
	"path/filepath"
	"strings"
)

// FindFile looks for a file with the given name (case-insensitive) in the specified directory.
// Returns the actual path with correct case if found, empty string if not found.
func FindFile(dir, filename string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		if strings.EqualFold(entry.Name(), filename) {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	return "", nil
}

// FindFiles looks for files matching any of the given names (case-insensitive) in the specified directory.
// Returns at most one file per directory to avoid duplicates on case-insensitive filesystems.
func FindFiles(dir string, filenames []string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var found []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		for _, filename := range filenames {
			if strings.EqualFold(entry.Name(), filename) {
				found = append(found, filepath.Join(dir, entry.Name()))
				goto nextEntry // Only match one file per entry to avoid duplicates
			}
		}
		nextEntry:
	}

	return found, nil
}