package filesystems

import (
	"iter"
	"strings"
)

// FindFile looks for a file with the given name (case-insensitive) in the provided directory entries.
// Returns the actual path with correct case if found, empty string if not found.
func FindFile(filesystem FileSystem, dir, filename string, entries iter.Seq2[DirEntry, error]) (string, error) {
	for entry, err := range entries {
		if err != nil {
			return "", err
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), filename) {
			return filesystem.Join(dir, entry.Name()), nil
		}
	}

	return "", nil
}
