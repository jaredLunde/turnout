package signals

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type DockerfileSignal struct {
	filesystem     fs.FileSystem
	dockerfiles    []string          // all found Dockerfiles
	dockerfileDirs map[string]string // dockerfile path -> directory path
}

func NewDockerfileSignal(filesystem fs.FileSystem) *DockerfileSignal {
	return &DockerfileSignal{filesystem: filesystem}
}

func (d *DockerfileSignal) Confidence() int {
	return 50 // Poor confidence - just indicates buildable service, not deployment config
}

func (d *DockerfileSignal) Reset() {
	d.dockerfiles = nil
	d.dockerfileDirs = make(map[string]string)
}

func (d *DockerfileSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if !entry.IsDir() {
		name := entry.Name()
		// Match standard Dockerfile
		if strings.EqualFold(name, "Dockerfile") {
			dockerfilePath := d.filesystem.Join(rootPath, name)
			d.dockerfiles = append(d.dockerfiles, dockerfilePath)
			d.dockerfileDirs[dockerfilePath] = rootPath
		}
		// Match named Dockerfiles: Dockerfile.*, *.Dockerfile
		if strings.HasPrefix(strings.ToLower(name), "dockerfile.") || strings.HasSuffix(strings.ToLower(name), ".dockerfile") {
			dockerfilePath := d.filesystem.Join(rootPath, name)
			d.dockerfiles = append(d.dockerfiles, dockerfilePath)
			d.dockerfileDirs[dockerfilePath] = rootPath
		}
	}

	return nil
}

func (d *DockerfileSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(d.dockerfiles) == 0 {
		return nil, nil
	}

	var services []types.Service
	for _, dockerfilePath := range d.dockerfiles {
		rootPath := d.dockerfileDirs[dockerfilePath]
		service := types.Service{
			Name:      d.inferServiceName(dockerfilePath, rootPath),
			Network:   types.NetworkPrivate, // Conservative default
			Runtime:   types.RuntimeContinuous,
			Build:     types.BuildFromSource,
			BuildPath: d.filesystem.Dir(dockerfilePath),
			Configs: []types.ConfigRef{
				{Type: "dockerfile", Path: dockerfilePath},
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
