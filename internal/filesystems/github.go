package filesystems

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v74/github"
	"golang.org/x/oauth2"
)

// GitHubFS implements FileSystem using downloaded GitHub repository archive
type GitHubFS struct {
	ctx        context.Context
	owner      string
	repo       string
	ref        string
	basePath   string
	repoPrefix string
	token      string
	initErr    error

	client    *github.Client
	zipReader *zip.ReadCloser
	pathIndex map[string][]string

	once sync.Once
}

// NewGitHubFS creates a new GitHubFS instance
func NewGitHubFS(owner, repo, ref string, token string) *GitHubFS {
	return NewGitHubFSWithPath(owner, repo, ref, "", token)
}

// NewGitHubFSWithPath creates a new GitHubFS instance with a base path
func NewGitHubFSWithPath(owner, repo, ref, basePath string, token string) *GitHubFS {
	ctx := context.Background()
	var client *github.Client

	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	} else {
		client = github.NewClient(nil)
	}

	if ref == "" {
		// Detect the actual default branch
		repo, _, err := client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			// Fallback to "main" if we can't detect
			ref = "main"
		} else {
			ref = repo.GetDefaultBranch()
		}
	}

	return &GitHubFS{
		client:    client,
		ctx:       ctx,
		owner:     owner,
		repo:      repo,
		ref:       ref,
		basePath:  basePath,
		token:     token,
		pathIndex: make(map[string][]string),
	}
}

// ensureInitialized downloads the GitHub repository archive once and indexes it
func (gfs *GitHubFS) ensureInitialized() error {
	gfs.once.Do(func() {
		gfs.initErr = gfs.downloadAndIndex()
	})
	return gfs.initErr
}

// downloadAndIndex downloads the repository as a zipball and indexes its contents
func (gfs *GitHubFS) downloadAndIndex() error {
	// Create temporary file for zip
	tempFile, err := os.CreateTemp("", fmt.Sprintf("turnout-github-%s-%s-*.zip", gfs.owner, gfs.repo))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Download zipball
	if err := gfs.downloadZipball(tempFile); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to download repository: %w", err)
	}

	// Close the temp file
	tempFile.Close()

	// Open zip reader
	zipReader, err := zip.OpenReader(tempFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	gfs.zipReader = zipReader

	// Find the repo prefix and build lightweight path index
	gfs.findRepoPrefix()
	gfs.buildPathIndex()

	return nil
}

// downloadZipball downloads the repository zipball
func (gfs *GitHubFS) downloadZipball(file *os.File) error {
	var url string

	if gfs.token != "" {
		// Use GitHub API endpoint for authenticated requests
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/zipball/%s", gfs.owner, gfs.repo, gfs.ref)
	} else {
		// For unauthenticated requests use codeload directly
		url = fmt.Sprintf("https://codeload.github.com/%s/%s/zip/%s", gfs.owner, gfs.repo, gfs.ref)
	}

	req, err := http.NewRequestWithContext(gfs.ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Add token to Authorization header if available
	if gfs.token != "" {
		req.Header.Set("Authorization", "Bearer "+gfs.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Don't leak token in error message
		safeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/zipball/%s", gfs.owner, gfs.repo, gfs.ref)
		if gfs.token == "" {
			safeURL = fmt.Sprintf("https://codeload.github.com/%s/%s/zip/%s", gfs.owner, gfs.repo, gfs.ref)
		}

		// Add more context for debugging
		statusText := http.StatusText(resp.StatusCode)
		return fmt.Errorf("failed to download archive: HTTP %d %s for %s", resp.StatusCode, statusText, safeURL)
	}

	_, err = io.Copy(file, resp.Body)
	return err
}

// findRepoPrefix determines the prefix GitHub adds to zip entries
func (gfs *GitHubFS) findRepoPrefix() {
	for _, f := range gfs.zipReader.File {
		if f.FileInfo().IsDir() {
			parts := strings.Split(strings.Trim(f.Name, "/"), "/")
			if len(parts) > 0 {
				gfs.repoPrefix = parts[0] + "/"
				break
			}
		}
	}
}

// buildPathIndex creates minimal directory index (just strings, not zip entries)
func (gfs *GitHubFS) buildPathIndex() {
	for _, f := range gfs.zipReader.File {
		// Remove repo prefix to get clean path
		cleanPath := strings.TrimPrefix(f.Name, gfs.repoPrefix)
		cleanPath = strings.Trim(cleanPath, "/")

		if cleanPath == "" {
			continue // Skip root
		}

		// Convert to forward slashes for consistency
		cleanPath = filepath.ToSlash(cleanPath)

		// Get parent directory and child name
		parentDir := path.Dir(cleanPath)
		if parentDir == "." {
			parentDir = ""
		}
		childName := path.Base(cleanPath)

		// Add child name to parent's list if not already present (just strings)
		children := gfs.pathIndex[parentDir]
		found := false
		for _, existing := range children {
			if existing == childName {
				found = true
				break
			}
		}
		if !found {
			gfs.pathIndex[parentDir] = append(children, childName)
		}
	}
}

// Add cleanup method
func (gfs *GitHubFS) Cleanup() error {
	if gfs.zipReader != nil {
		return gfs.zipReader.Close()
	}
	return nil
}

// validatePath ensures the path is safe and within bounds
func (gfs *GitHubFS) validatePath(path string) error {
	// Clean and normalize path
	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)

	// Reject dangerous paths
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute path not allowed: %s", path)
	}
	if strings.HasPrefix(path, "../") || path == ".." {
		return fmt.Errorf("parent directory access not allowed: %s", path)
	}

	return nil
}

func (gfs *GitHubFS) ReadFile(name string) ([]byte, error) {
	if err := gfs.ensureInitialized(); err != nil {
		return nil, err
	}

	// Validate and resolve path
	if err := gfs.validatePath(name); err != nil {
		return nil, err
	}

	name = gfs.resolvePath(name)

	// Use zip reader's built-in Open method (has internal indexing)
	targetPath := gfs.repoPrefix + name
	file, err := gfs.zipReader.Open(targetPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", name)
	}
	defer file.Close()

	return io.ReadAll(file)
}

func (gfs *GitHubFS) ReadDir(name string) iter.Seq2[DirEntry, error] {
	return func(yield func(DirEntry, error) bool) {
		if err := gfs.ensureInitialized(); err != nil {
			yield(nil, err)
			return
		}

		// Validate and resolve path
		if err := gfs.validatePath(name); err != nil {
			yield(nil, err)
			return
		}

		name = gfs.resolvePath(name)

		// Handle root directory special case
		if name == "." {
			name = ""
		}

		// Get children from minimal path index (just strings)
		children, exists := gfs.pathIndex[name]
		if !exists {
			yield(nil, fmt.Errorf("directory not found: %s", name))
			return
		}

		// Yield lightweight DirEntry objects without allocating slice
		for _, childName := range children {
			childPath := name
			if childPath != "" {
				childPath += "/"
			}
			childPath += childName

			_, isDir := gfs.pathIndex[childPath]

			entry := &lightweightDirEntry{
				name:       childName,
				isDir:      isDir,
				parentPath: name,
			}
			if !yield(entry, nil) {
				return // Consumer stopped iteration
			}
		}
	}
}

// lightweightDirEntry implements DirEntry without holding zip.File references
type lightweightDirEntry struct {
	name       string
	parentPath string
	isDir      bool
}

func (e *lightweightDirEntry) Name() string {
	return e.name
}

func (e *lightweightDirEntry) IsDir() bool {
	return e.isDir
}

func (e *lightweightDirEntry) Type() fs.FileMode {
	if e.IsDir() {
		return fs.ModeDir
	}
	return 0
}

func (e *lightweightDirEntry) Info() (FileInfo, error) {
	return &lightweightFileInfo{
		name:  e.name,
		isDir: e.isDir,
	}, nil
}

// lightweightFileInfo implements FileInfo without zip.File references
type lightweightFileInfo struct {
	name  string
	isDir bool
}

func (fi *lightweightFileInfo) Name() string { return fi.name }
func (fi *lightweightFileInfo) Size() int64  { return 0 } // We don't need size for service discovery
func (fi *lightweightFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (fi *lightweightFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *lightweightFileInfo) IsDir() bool        { return fi.isDir }
func (fi *lightweightFileInfo) Sys() interface{}   { return nil }

// zipFileInfo wraps zip.File to implement FileInfo
type zipFileInfo struct {
	*zip.File
	name  string
	isDir bool
}

func (fi *zipFileInfo) Name() string {
	if fi.name != "" {
		return fi.name
	}
	if fi.File != nil {
		return filepath.Base(fi.File.Name)
	}
	return ""
}

func (fi *zipFileInfo) Size() int64 {
	if fi.File != nil {
		return int64(fi.File.UncompressedSize64)
	}
	return 0
}

func (fi *zipFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0755
	}
	if fi.File != nil {
		return fi.File.FileInfo().Mode()
	}
	return 0644
}

func (fi *zipFileInfo) ModTime() time.Time {
	if fi.File != nil {
		return fi.File.FileInfo().ModTime()
	}
	return time.Time{}
}

func (fi *zipFileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *zipFileInfo) Sys() interface{} {
	return fi.File
}

func (gfs *GitHubFS) resolvePath(path string) string {
	// Clean the path - remove leading slash if present
	path = strings.TrimPrefix(path, "/")

	// If we have a basePath, prepend it to the path
	if gfs.basePath != "" {
		if path == "" || path == "." {
			return gfs.basePath
		}
		return gfs.basePath + "/" + path
	}

	return path
}

func (gfs *GitHubFS) Walk(root string, fn WalkFunc) error {
	if err := gfs.ensureInitialized(); err != nil {
		return err
	}

	// Create root directory info
	rootInfo := &lightweightFileInfo{name: root, isDir: true}

	return gfs.walkRecursive(root, rootInfo, fn, 0, 10)
}

func (gfs *GitHubFS) walkRecursive(dir string, info FileInfo, fn WalkFunc, depth, maxDepth int) error {
	if depth > maxDepth {
		return nil
	}

	// Call fn for the directory itself
	if err := fn(dir, info, nil); err != nil {
		if err == SkipDir && info.IsDir() {
			return nil
		}
		return err
	}

	// If it's not a directory, we're done
	if !info.IsDir() {
		return nil
	}

	// Process directory contents using iterator
	for entry, err := range gfs.ReadDir(dir) {
		if err != nil {
			return fn(dir, info, err)
		}
		entryPath := gfs.Join(dir, entry.Name())
		entryInfo, err := entry.Info()
		if err != nil {
			if err := fn(entryPath, nil, err); err != nil {
				return err
			}
			continue
		}

		if err := fn(entryPath, entryInfo, nil); err != nil {
			if err == SkipDir && entry.IsDir() {
				continue
			}
			return err
		}

		// Recurse into subdirectories using the info we already have
		if entry.IsDir() {
			if err := gfs.walkRecursive(entryPath, entryInfo, fn, depth+1, maxDepth); err != nil {
				return err
			}
		}
	}

	return nil
}

func (gfs *GitHubFS) Join(elem ...string) string {
	return path.Join(elem...)
}

func (gfs *GitHubFS) Base(p string) string {
	return path.Base(p)
}

func (gfs *GitHubFS) Dir(p string) string {
	return path.Dir(p)
}

func (gfs *GitHubFS) Rel(basepath, targpath string) (string, error) {
	// Simple implementation for URL paths
	if strings.HasPrefix(targpath, basepath) {
		rel := strings.TrimPrefix(targpath, basepath)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return ".", nil
		}
		return rel, nil
	}
	return targpath, nil
}
