package fs

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

const (
	// MaxCacheMemory is the maximum memory usage for the cache (100MB)
	MaxCacheMemory = 100 * 1024 * 1024
)

// cacheEntry represents a cached item with its memory usage
type cacheEntry struct {
	data      interface{}
	size      int64
	timestamp time.Time
}

// memoryCache implements a bounded in-memory cache
type memoryCache struct {
	mu          sync.RWMutex
	entries     map[string]*cacheEntry
	totalMemory int64
	maxMemory   int64
}

func newMemoryCache(maxMemory int64) *memoryCache {
	return &memoryCache{
		entries:   make(map[string]*cacheEntry),
		maxMemory: maxMemory,
	}
}

func (c *memoryCache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}
	
	return entry.data, true
}

func (c *memoryCache) put(key string, data interface{}, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// If this entry already exists, remove its memory usage
	if existing, exists := c.entries[key]; exists {
		c.totalMemory -= existing.size
	}
	
	// Evict entries if we would exceed memory limit
	for c.totalMemory+size > c.maxMemory && len(c.entries) > 0 {
		c.evictOldest()
	}
	
	// Add the new entry
	c.entries[key] = &cacheEntry{
		data:      data,
		size:      size,
		timestamp: time.Now(),
	}
	c.totalMemory += size
}

func (c *memoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, entry := range c.entries {
		if oldestKey == "" || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
		}
	}
	
	if oldestKey != "" {
		c.totalMemory -= c.entries[oldestKey].size
		delete(c.entries, oldestKey)
	}
}

// GitHubFS implements FileSystem using downloaded GitHub repository archive
type GitHubFS struct {
	client     *github.Client
	ctx        context.Context
	owner      string
	repo       string
	ref        string // branch, tag, or commit SHA
	basePath   string // optional subdirectory to start from
	zipReader  *zip.ReadCloser
	repoPrefix string // the prefix GitHub adds to zip entries (e.g., "repo-main/")
	once       sync.Once
	initErr    error
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
		ref = "main" // default branch
	}
	
	return &GitHubFS{
		client:   client,
		ctx:      ctx,
		owner:    owner,
		repo:     repo,
		ref:      ref,
		basePath: basePath,
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
	
	// Find the repo prefix by looking at first entry
	gfs.findRepoPrefix()
	
	return nil
}

// downloadZipball downloads the repository zipball
func (gfs *GitHubFS) downloadZipball(file *os.File) error {
	// Use GitHub archive URL: https://github.com/owner/repo/archive/ref.zip
	url := fmt.Sprintf("https://github.com/%s/%s/archive/%s.zip", gfs.owner, gfs.repo, gfs.ref)
	
	req, err := http.NewRequestWithContext(gfs.ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	// Add authentication if available
	if gfs.client != nil {
		// Try to get token from client
		if transport, ok := gfs.client.Client().Transport.(*oauth2.Transport); ok {
			if token, err := transport.Source.Token(); err == nil && token.AccessToken != "" {
				req.Header.Set("Authorization", "token "+token.AccessToken)
			}
		}
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download archive: HTTP %d", resp.StatusCode)
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
	
	// Find file by scanning zip entries
	targetPath := gfs.repoPrefix + name
	for _, f := range gfs.zipReader.File {
		if !f.FileInfo().IsDir() && f.Name == targetPath {
			reader, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer reader.Close()
			return io.ReadAll(reader)
		}
	}
	
	return nil, fmt.Errorf("file not found: %s", name)
}

func (gfs *GitHubFS) ReadDir(name string) ([]DirEntry, error) {
	if err := gfs.ensureInitialized(); err != nil {
		return nil, err
	}
	
	// Validate and resolve path
	if err := gfs.validatePath(name); err != nil {
		return nil, err
	}
	
	name = gfs.resolvePath(name)
	
	// Build target directory prefix
	targetPrefix := gfs.repoPrefix
	if name != "" && name != "." {
		targetPrefix += name + "/"
	}
	
	// Scan zip entries to find direct children of this directory
	var entries []DirEntry
	seen := make(map[string]bool)
	
	for _, f := range gfs.zipReader.File {
		// Skip if not in target directory
		if !strings.HasPrefix(f.Name, targetPrefix) {
			continue
		}
		
		// Get relative path within target directory
		relPath := strings.TrimPrefix(f.Name, targetPrefix)
		if relPath == "" {
			continue // Skip the directory itself
		}
		
		// For direct children, get first path component
		parts := strings.SplitN(strings.Trim(relPath, "/"), "/", 2)
		if len(parts) == 0 {
			continue
		}
		
		childName := parts[0]
		if childName == "" {
			continue
		}
		
		// Skip if we've already seen this child
		if seen[childName] {
			continue
		}
		seen[childName] = true
		
		// This entry represents a direct child - add it
		entries = append(entries, &zipDirEntry{f})
	}
	
	return entries, nil
}

func (gfs *GitHubFS) Stat(name string) (FileInfo, error) {
	if err := gfs.ensureInitialized(); err != nil {
		return nil, err
	}
	
	// Validate path
	if err := gfs.validatePath(name); err != nil {
		return nil, err
	}
	
	originalName := name
	name = gfs.resolvePath(name)
	
	// Special case for root directory
	if originalName == "." || name == "" {
		return &zipFileInfo{nil, true, "."}, nil
	}
	
	// Build target path
	targetPath := gfs.repoPrefix + name
	
	// Scan zip entries to find exact match
	for _, f := range gfs.zipReader.File {
		if f.Name == targetPath {
			return &zipFileInfo{f, f.FileInfo().IsDir(), ""}, nil
		}
		// Also check with trailing slash for directories
		if f.Name == targetPath+"/" {
			return &zipFileInfo{f, true, ""}, nil
		}
	}
	
	// Check if it's an implicit directory (has children)
	targetPrefix := targetPath + "/"
	for _, f := range gfs.zipReader.File {
		if strings.HasPrefix(f.Name, targetPrefix) {
			return &zipFileInfo{nil, true, filepath.Base(name)}, nil
		}
	}
	
	return nil, fmt.Errorf("path not found: %s", name)
}

// zipDirEntry wraps zip.File to implement DirEntry
type zipDirEntry struct {
	*zip.File
}

func (e *zipDirEntry) Name() string {
	name := strings.TrimSuffix(filepath.Base(e.File.Name), "/")
	return name
}

func (e *zipDirEntry) IsDir() bool {
	return e.File.FileInfo().IsDir()
}

func (e *zipDirEntry) Type() fs.FileMode {
	if e.IsDir() {
		return fs.ModeDir
	}
	return 0
}

func (e *zipDirEntry) Info() (FileInfo, error) {
	return &zipFileInfo{e.File, false, ""}, nil
}

// zipFileInfo wraps zip.File to implement FileInfo
type zipFileInfo struct {
	*zip.File
	isDir bool
	name  string
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
	
	// For the root directory, we need to get its info first
	rootInfo, err := gfs.Stat(root)
	if err != nil {
		return fn(root, nil, err)
	}
	
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
	
	// Read directory contents
	entries, err := gfs.ReadDir(dir)
	if err != nil {
		return fn(dir, info, err)
	}
	
	for _, entry := range entries {
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
