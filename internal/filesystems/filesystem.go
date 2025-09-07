package filesystems

import (
	"io/fs"
	"iter"
	"time"
)

// FileSystem abstracts filesystem operations for different backends
type FileSystem interface {
	// ReadFile reads the named file and returns its contents
	ReadFile(name string) ([]byte, error)

	// ReadDir reads the named directory and returns an iterator over directory entries
	ReadDir(name string) iter.Seq2[DirEntry, error]

	// Walk walks the file tree rooted at root, calling fn for each file or directory
	Walk(root string, fn WalkFunc) error

	// Join joins path elements into a single path
	Join(elem ...string) string

	// Base returns the last element of path
	Base(path string) string

	// Dir returns all but the last element of path
	Dir(path string) string

	// Rel returns a relative path from basepath to targpath
	Rel(basepath, targpath string) (string, error)
}

// DirEntry provides information about a directory entry
type DirEntry interface {
	Name() string
	IsDir() bool
	Type() fs.FileMode
	Info() (FileInfo, error)
}

// FileInfo provides information about a file
type FileInfo interface {
	Name() string
	Size() int64
	Mode() fs.FileMode
	ModTime() time.Time
	IsDir() bool
	Sys() interface{}
}

// WalkFunc is the type of function called by Walk
type WalkFunc func(path string, info FileInfo, err error) error

// SkipDir is used as a return value from WalkFunc to indicate that
// the directory named in the call is to be skipped
var SkipDir = fs.SkipDir
