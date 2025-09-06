package fs

import (
	"iter"
	"os"
	"path/filepath"
)

// LocalFS implements FileSystem for local filesystem access
type LocalFS struct{}

// NewLocalFS creates a new LocalFS instance
func NewLocalFS() *LocalFS {
	return &LocalFS{}
}

func (lfs *LocalFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (lfs *LocalFS) ReadDir(name string) iter.Seq2[DirEntry, error] {
	return func(yield func(DirEntry, error) bool) {
		entries, err := os.ReadDir(name)
		if err != nil {
			yield(nil, err)
			return
		}
		
		for _, entry := range entries {
			localEntry := &localDirEntry{entry}
			if !yield(localEntry, nil) {
				return // Consumer stopped iteration
			}
		}
	}
}

func (lfs *LocalFS) Stat(name string) (FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	return &localFileInfo{info}, nil
}

func (lfs *LocalFS) Walk(root string, fn WalkFunc) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		var fileInfo FileInfo
		if info != nil {
			fileInfo = &localFileInfo{info}
		}
		return fn(path, fileInfo, err)
	})
}

func (lfs *LocalFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (lfs *LocalFS) Base(path string) string {
	return filepath.Base(path)
}

func (lfs *LocalFS) Dir(path string) string {
	return filepath.Dir(path)
}

func (lfs *LocalFS) Rel(basepath, targpath string) (string, error) {
	return filepath.Rel(basepath, targpath)
}

// localDirEntry wraps os.DirEntry
type localDirEntry struct {
	os.DirEntry
}

func (e *localDirEntry) Info() (FileInfo, error) {
	info, err := e.DirEntry.Info()
	if err != nil {
		return nil, err
	}
	return &localFileInfo{info}, nil
}

// localFileInfo wraps os.FileInfo
type localFileInfo struct {
	os.FileInfo
}