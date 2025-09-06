package fs

import (
	"strings"
)

// FindFile looks for a file with the given name (case-insensitive) in the specified directory using the provided filesystem.
// Returns the actual path with correct case if found, empty string if not found.
func FindFile(filesystem FileSystem, dir, filename string) (string, error) {
	entries, err := filesystem.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		if strings.EqualFold(entry.Name(), filename) {
			return filesystem.Join(dir, entry.Name()), nil
		}
	}

	return "", nil
}

// FindFiles looks for files matching any of the given names (case-insensitive) in the specified directory.
// Returns at most one file per directory to avoid duplicates on case-insensitive filesystems.
// Legacy function for backwards compatibility - uses local filesystem
func FindFiles(dir string, filenames []string) ([]string, error) {
	localFS := NewLocalFS()
	entries, err := localFS.ReadDir(dir)
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
				found = append(found, localFS.Join(dir, entry.Name()))
				goto nextEntry // Only match one file per entry to avoid duplicates
			}
		}
		nextEntry:
	}

	return found, nil
}

// FindFileInEntries looks for a file with the given name (case-insensitive) in the provided directory entries.
// Returns the actual path with correct case if found, empty string if not found.
func FindFileInEntries(filesystem FileSystem, dir, filename string, entries []DirEntry) (string, error) {
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), filename) {
			return filesystem.Join(dir, entry.Name()), nil
		}
	}
	
	return "", nil
}