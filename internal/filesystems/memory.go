package filesystems

import (
	"fmt"
	"io/fs"
	"iter"
	"path"
	"sort"
	"strings"
	"time"
)

// MemoryFS implements FileSystem for in-memory filesystem operations
type MemoryFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

// NewMemoryFS creates a new MemoryFS instance
func NewMemoryFS() *MemoryFS {
	return &MemoryFS{
		files: make(map[string][]byte),
		dirs:  make(map[string]bool),
	}
}

// AddFile adds a file to the memory filesystem
func (mfs *MemoryFS) AddFile(name string, content []byte) {
	mfs.files[path.Clean(name)] = content
	// Ensure parent directories exist
	dir := path.Dir(name)
	for dir != "." && dir != "/" {
		mfs.dirs[dir] = true
		dir = path.Dir(dir)
	}
}

// AddDir adds a directory to the memory filesystem
func (mfs *MemoryFS) AddDir(name string) {
	mfs.dirs[path.Clean(name)] = true
	// Ensure parent directories exist
	dir := path.Dir(name)
	for dir != "." && dir != "/" {
		mfs.dirs[dir] = true
		dir = path.Dir(dir)
	}
}

func (mfs *MemoryFS) ReadFile(name string) ([]byte, error) {
	cleanName := path.Clean(name)
	content, exists := mfs.files[cleanName]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", name)
	}
	return content, nil
}

func (mfs *MemoryFS) ReadDir(name string) iter.Seq2[DirEntry, error] {
	return func(yield func(DirEntry, error) bool) {
		cleanName := path.Clean(name)

		// Check if directory exists
		if cleanName != "." && !mfs.dirs[cleanName] {
			yield(nil, fmt.Errorf("directory not found: %s", name))
			return
		}

		entries := make([]string, 0)
		prefix := cleanName
		if prefix != "." {
			prefix += "/"
		}

		// Collect direct children
		seen := make(map[string]bool)

		// Check files
		for filePath := range mfs.files {
			if strings.HasPrefix(filePath, prefix) || (cleanName == "." && !strings.Contains(filePath, "/")) {
				remainder := filePath
				if cleanName != "." {
					remainder = strings.TrimPrefix(filePath, prefix)
				}
				if remainder != "" {
					parts := strings.Split(remainder, "/")
					childName := parts[0]
					if !seen[childName] {
						entries = append(entries, childName)
						seen[childName] = true
					}
				}
			}
		}

		// Check directories
		for dirPath := range mfs.dirs {
			if strings.HasPrefix(dirPath, prefix) || (cleanName == "." && !strings.Contains(dirPath, "/")) {
				remainder := dirPath
				if cleanName != "." {
					remainder = strings.TrimPrefix(dirPath, prefix)
				}
				if remainder != "" {
					parts := strings.Split(remainder, "/")
					childName := parts[0]
					if !seen[childName] {
						entries = append(entries, childName)
						seen[childName] = true
					}
				}
			}
		}

		sort.Strings(entries)

		for _, entry := range entries {
			fullPath := entry
			if cleanName != "." {
				fullPath = path.Join(cleanName, entry)
			}

			isDir := mfs.dirs[fullPath]
			if !isDir {
				_, isFile := mfs.files[fullPath]
				if !isFile {
					// This is a directory that contains other files/dirs
					isDir = true
				}
			}

			dirEntry := &memoryDirEntry{
				name:     entry,
				isDir:    isDir,
				mfs:      mfs,
				fullPath: fullPath,
			}

			if !yield(dirEntry, nil) {
				return
			}
		}
	}
}

func (mfs *MemoryFS) Walk(root string, fn WalkFunc) error {
	cleanRoot := path.Clean(root)

	var walkDir func(string) error
	walkDir = func(dir string) error {
		// Process current directory
		isDir := mfs.dirs[dir] || dir == "."
		var fileInfo FileInfo
		if isDir {
			fileInfo = &memoryFileInfo{
				name:    path.Base(dir),
				size:    0,
				mode:    fs.ModeDir | 0755,
				modTime: time.Now(),
				isDir:   true,
			}
		} else if content, exists := mfs.files[dir]; exists {
			fileInfo = &memoryFileInfo{
				name:    path.Base(dir),
				size:    int64(len(content)),
				mode:    0644,
				modTime: time.Now(),
				isDir:   false,
			}
		}

		if fileInfo != nil {
			if err := fn(dir, fileInfo, nil); err != nil {
				if err == SkipDir && fileInfo.IsDir() {
					return nil
				}
				return err
			}
		}

		// If it's a directory, walk its children
		if isDir {
			for entry, _ := range mfs.ReadDir(dir) {
				if entry != nil {
					childPath := path.Join(dir, entry.Name())
					if err := walkDir(childPath); err != nil {
						return err
					}
				}
			}
		}

		return nil
	}

	return walkDir(cleanRoot)
}

func (mfs *MemoryFS) Join(elem ...string) string {
	return path.Join(elem...)
}

func (mfs *MemoryFS) Base(p string) string {
	return path.Base(p)
}

func (mfs *MemoryFS) Dir(p string) string {
	return path.Dir(p)
}

func (mfs *MemoryFS) Rel(basepath, targpath string) (string, error) {
	// Simple implementation - for more complex cases, this would need enhancement
	base := path.Clean(basepath)
	target := path.Clean(targpath)

	if base == target {
		return ".", nil
	}

	// If target starts with base, return relative path
	if strings.HasPrefix(target, base+"/") {
		return strings.TrimPrefix(target, base+"/"), nil
	}

	// For other cases, return the target as-is (simplified)
	return target, nil
}

// memoryDirEntry implements DirEntry for memory filesystem
type memoryDirEntry struct {
	name     string
	isDir    bool
	mfs      *MemoryFS
	fullPath string
}

func (e *memoryDirEntry) Name() string {
	return e.name
}

func (e *memoryDirEntry) IsDir() bool {
	return e.isDir
}

func (e *memoryDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}

func (e *memoryDirEntry) Info() (FileInfo, error) {
	if e.isDir {
		return &memoryFileInfo{
			name:    e.name,
			size:    0,
			mode:    fs.ModeDir | 0755,
			modTime: time.Now(),
			isDir:   true,
		}, nil
	}

	content, exists := e.mfs.files[e.fullPath]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", e.fullPath)
	}

	return &memoryFileInfo{
		name:    e.name,
		size:    int64(len(content)),
		mode:    0644,
		modTime: time.Now(),
		isDir:   false,
	}, nil
}

// memoryFileInfo implements FileInfo for memory filesystem
type memoryFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *memoryFileInfo) Name() string {
	return fi.name
}

func (fi *memoryFileInfo) Size() int64 {
	return fi.size
}

func (fi *memoryFileInfo) Mode() fs.FileMode {
	return fi.mode
}

func (fi *memoryFileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *memoryFileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *memoryFileInfo) Sys() interface{} {
	return nil
}
