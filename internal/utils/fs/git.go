package fs

import (
	"fmt"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// GitFS implements FileSystem for git repositories (cloned locally)
type GitFS struct {
	repoURL   string
	ref       string
	localPath string
	localFS   *LocalFS
	mu        sync.RWMutex // protects clone operations
	cloned    bool
}

// NewGitFS creates a new GitFS instance
func NewGitFS(repoURL, ref string) (*GitFS, error) {
	if ref == "" {
		ref = "main" // default branch
	}
	
	// Create temporary directory for cloning
	tempDir, err := os.MkdirTemp("", "turnout-git-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	gfs := &GitFS{
		repoURL:   repoURL,
		ref:       ref,
		localPath: tempDir,
		localFS:   NewLocalFS(),
	}
	
	// Clone the repository
	if err := gfs.clone(); err != nil {
		os.RemoveAll(tempDir) // cleanup on error
		return nil, err
	}
	
	return gfs, nil
}

func (gfs *GitFS) clone() error {
	gfs.mu.Lock()
	defer gfs.mu.Unlock()
	
	if gfs.cloned {
		return nil
	}
	
	// Clone with depth 1 for performance
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", gfs.ref, gfs.repoURL, gfs.localPath)
	if err := cmd.Run(); err != nil {
		// If branch clone fails, try without branch specification
		cmd = exec.Command("git", "clone", "--depth", "1", gfs.repoURL, gfs.localPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository %s: %w", gfs.repoURL, err)
		}
		
		// Try to checkout the specific ref
		cmd = exec.Command("git", "checkout", gfs.ref)
		cmd.Dir = gfs.localPath
		if err := cmd.Run(); err != nil {
			// If checkout fails, continue with default branch
			// This handles cases where ref doesn't exist
		}
	}
	
	gfs.cloned = true
	return nil
}

// Cleanup removes the temporary git repository
func (gfs *GitFS) Cleanup() error {
	if gfs.localPath != "" {
		return os.RemoveAll(gfs.localPath)
	}
	return nil
}

func (gfs *GitFS) ReadFile(name string) ([]byte, error) {
	if err := gfs.ensureCloned(); err != nil {
		return nil, err
	}
	
	fullPath := gfs.localFS.Join(gfs.localPath, name)
	return gfs.localFS.ReadFile(fullPath)
}

func (gfs *GitFS) ReadDir(name string) iter.Seq2[DirEntry, error] {
	return func(yield func(DirEntry, error) bool) {
		if err := gfs.ensureCloned(); err != nil {
			yield(nil, err)
			return
		}
		
		fullPath := gfs.localFS.Join(gfs.localPath, name)
		for entry, err := range gfs.localFS.ReadDir(fullPath) {
			if !yield(entry, err) {
				return
			}
		}
	}
}

func (gfs *GitFS) Stat(name string) (FileInfo, error) {
	if err := gfs.ensureCloned(); err != nil {
		return nil, err
	}
	
	fullPath := gfs.localFS.Join(gfs.localPath, name)
	return gfs.localFS.Stat(fullPath)
}

func (gfs *GitFS) Walk(root string, fn WalkFunc) error {
	if err := gfs.ensureCloned(); err != nil {
		return err
	}
	
	fullRoot := gfs.localFS.Join(gfs.localPath, root)
	
	// Wrap the walk function to adjust paths
	wrappedFn := func(path string, info FileInfo, err error) error {
		// Convert absolute path back to relative path
		if strings.HasPrefix(path, gfs.localPath) {
			relPath := strings.TrimPrefix(path, gfs.localPath)
			relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
			if relPath == "" {
				relPath = "."
			}
			return fn(relPath, info, err)
		}
		return fn(path, info, err)
	}
	
	return gfs.localFS.Walk(fullRoot, wrappedFn)
}

func (gfs *GitFS) Join(elem ...string) string {
	return gfs.localFS.Join(elem...)
}

func (gfs *GitFS) Base(path string) string {
	return gfs.localFS.Base(path)
}

func (gfs *GitFS) Dir(path string) string {
	return gfs.localFS.Dir(path)
}

func (gfs *GitFS) Rel(basepath, targpath string) (string, error) {
	return gfs.localFS.Rel(basepath, targpath)
}

func (gfs *GitFS) ensureCloned() error {
	gfs.mu.RLock()
	cloned := gfs.cloned
	gfs.mu.RUnlock()
	
	if !cloned {
		return gfs.clone()
	}
	return nil
}