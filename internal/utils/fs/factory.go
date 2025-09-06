package fs

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// NewFileSystem creates a filesystem implementation based on the given URI
// Supports:
// - file:///path/to/local/dir
// - github://owner/repo/tree/branch
// - git://github.com/owner/repo
func NewFileSystem(uri string) (FileSystem, error) {
	// Handle local paths without scheme
	if !strings.Contains(uri, "://") {
		// Convert to absolute path for validation
		_, err := filepath.Abs(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %w", uri, err)
		}
		return NewLocalFS(), nil
	}
	
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI %s: %w", uri, err)
	}
	
	switch parsedURL.Scheme {
	case "file":
		return NewLocalFS(), nil
		
	case "github":
		return parseGitHubURL(parsedURL)
		
	case "git":
		return parseGitURL(parsedURL)
		
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}
}

// parseGitHubURL parses github://owner/repo/tree/branch URLs
func parseGitHubURL(u *url.URL) (FileSystem, error) {
	// Format: github://owner/repo/tree/branch
	// Or: github://owner/repo (defaults to main branch)
	
	// The host should be the owner for github:// URLs
	owner := u.Host
	
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	
	
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("invalid GitHub URL format, expected: github://owner/repo[/tree/branch]")
	}
	
	repo := parts[0]
	ref := ""
	
	// Check if tree/branch is specified
	if len(parts) >= 3 && parts[1] == "tree" {
		ref = parts[2]
		// Check if there's a subpath after the branch
		if len(parts) > 3 {
			subpath := strings.Join(parts[3:], "/")
			return NewGitHubFSWithPath(owner, repo, ref, subpath, os.Getenv("GITHUB_TOKEN")), nil
		}
	}
	
	// Get GitHub token from environment
	token := os.Getenv("GITHUB_TOKEN")
	
	return NewGitHubFS(owner, repo, ref, token), nil
}

// parseGitURL parses git://github.com/owner/repo URLs
func parseGitURL(u *url.URL) (FileSystem, error) {
	// Format: git://github.com/owner/repo
	// Or: git://github.com/owner/repo#branch
	
	// Reconstruct the full git URL
	gitURL := fmt.Sprintf("https://%s%s", u.Host, u.Path)
	
	ref := ""
	if u.Fragment != "" {
		ref = u.Fragment
	}
	
	gitFS, err := NewGitFS(gitURL, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to create git filesystem: %w", err)
	}
	
	return gitFS, nil
}

// GetBasePath returns the base path for the given URI
// This is useful for resolving relative paths in the CLI
func GetBasePath(uri string) string {
	if !strings.Contains(uri, "://") {
		return uri
	}
	
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	
	switch parsedURL.Scheme {
	case "file":
		return parsedURL.Path
		
	case "github":
		// For GitHub URLs, the filesystem handles subpaths internally
		return "."
		
	case "git":
		// For git URLs, the base path is "."
		return "."
		
	default:
		return uri
	}
}