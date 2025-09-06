package test

import (
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sync/singleflight"
)

var (
	cloneGroup singleflight.Group
)

// GetTestRepo clones or returns cached path to a test repository
// Uses singleflight to ensure only one clone per repo URL across all goroutines
func GetTestRepo(repoURL string) (string, error) {
	result, err, _ := cloneGroup.Do(repoURL, func() (interface{}, error) {
		return cloneRepo(repoURL)
	})

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

func cloneRepo(repoURL string) (string, error) {
	// Use a deterministic directory name based on repo URL
	repoName := filepath.Base(repoURL)
	if filepath.Ext(repoName) == ".git" {
		repoName = repoName[:len(repoName)-4]
	}

	// Put in system cache directory
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}

	repoDir := filepath.Join(cacheDir, ".turnout/testdata", repoName)

	// Check if repo already exists
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		// Repo exists, try to update it
		cmd := exec.Command("git", "pull")
		cmd.Dir = repoDir
		_ = cmd.Run() // Ignore errors, use existing repo
		return repoDir, nil
	}

	// Clone fresh repo
	if err := os.MkdirAll(filepath.Dir(repoDir), 0755); err != nil {
		return "", err
	}

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, repoDir)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return repoDir, nil
}
