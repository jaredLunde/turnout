package signals

import (
	"context"
	"iter"
	"path/filepath"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type DockerfileSignal struct {
	filesystem fs.FileSystem
}

func NewDockerfileSignal(filesystem fs.FileSystem) *DockerfileSignal {
	return &DockerfileSignal{filesystem: filesystem}
}

func (d *DockerfileSignal) Confidence() int {
	return 70 // Moderate confidence - just indicates buildable service, not deployment config
}

func (d *DockerfileSignal) Discover(ctx context.Context, rootPath string, dirEntries iter.Seq2[fs.DirEntry, error]) ([]types.Service, error) {
	var services []types.Service

	// Check for Dockerfile in current directory
	if found, err := fs.FindFileInEntries(d.filesystem, rootPath, "Dockerfile", dirEntries); err == nil && found != "" {
		service := types.Service{
			Name:      d.inferServiceName(found, rootPath),
			Network:   types.NetworkPrivate, // Conservative default
			Runtime:   types.RuntimeContinuous,
			Build:     types.BuildFromSource,
			BuildPath: d.filesystem.Dir(found),
			Configs: []types.ConfigRef{
				{Type: "dockerfile", Path: found},
			},
		}

		services = append(services, service)
	}

	return services, nil
}


func (d *DockerfileSignal) inferServiceName(dockerfilePath, rootPath string) string {
	dir := d.filesystem.Dir(dockerfilePath)
	
	// If Dockerfile is in root, use root directory name
	if dir == rootPath {
		return d.filesystem.Base(rootPath)
	}
	
	// Use subdirectory name
	rel, err := d.filesystem.Rel(rootPath, dir)
	if err != nil {
		return d.filesystem.Base(dir)
	}
	
	// Use first directory component as service name
	parts := strings.Split(rel, string(filepath.Separator))
	return parts[0]
}