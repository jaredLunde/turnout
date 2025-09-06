package signals

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type DockerfileSignal struct{}

func (d *DockerfileSignal) Confidence() int {
	return 80 // High confidence - Dockerfile indicates a buildable service
}

func (d *DockerfileSignal) Discover(ctx context.Context, rootPath string) ([]types.Service, error) {
	var services []types.Service

	// Look for Dockerfiles in root and immediate subdirectories
	dockerfiles, err := findDockerfiles(rootPath)
	if err != nil {
		return nil, err
	}

	for _, dockerfilePath := range dockerfiles {
		service := types.Service{
			Name:      inferServiceName(dockerfilePath, rootPath),
			Network:   types.NetworkPrivate, // Conservative default
			Runtime:   types.RuntimeContinuous,
			Build:     types.BuildFromSource,
			BuildPath: filepath.Dir(dockerfilePath),
			Configs: []types.ConfigRef{
				{Type: "dockerfile", Path: dockerfilePath},
			},
		}

		services = append(services, service)
	}

	return services, nil
}

func findDockerfiles(rootPath string) ([]string, error) {
	var dockerfiles []string

	// Check root directory
	if found, err := fs.FindFile(rootPath, "Dockerfile"); err == nil && found != "" {
		dockerfiles = append(dockerfiles, found)
	}

	// Check immediate subdirectories
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return dockerfiles, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(rootPath, entry.Name())
			if found, err := fs.FindFile(subdir, "Dockerfile"); err == nil && found != "" {
				dockerfiles = append(dockerfiles, found)
			}
		}
	}

	return dockerfiles, nil
}

func inferServiceName(dockerfilePath, rootPath string) string {
	dir := filepath.Dir(dockerfilePath)
	
	// If Dockerfile is in root, use root directory name
	if dir == rootPath {
		return filepath.Base(rootPath)
	}
	
	// Use subdirectory name
	rel, err := filepath.Rel(rootPath, dir)
	if err != nil {
		return filepath.Base(dir)
	}
	
	// Use first directory component as service name
	parts := strings.Split(rel, string(filepath.Separator))
	return parts[0]
}