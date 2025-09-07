package signals

import (
	"context"
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type DockerComposeSignal struct {
	filesystem   fs.FileSystem
	composeFiles []string          // all found compose files
	composeDirs  map[string]string // compose file path -> directory path
}

func NewDockerComposeSignal(filesystem fs.FileSystem) *DockerComposeSignal {
	return &DockerComposeSignal{filesystem: filesystem}
}

func (d *DockerComposeSignal) Confidence() int {
	return 80 // High confidence - but often used for local dev, not production deployment
}

func (d *DockerComposeSignal) Reset() {
	d.composeFiles = nil
	d.composeDirs = make(map[string]string)
}

var composeFiles = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
	"docker-compose.prod.yml",
	"docker-compose.prod.yaml",
	"docker-compose.production.yml",
	"docker-compose.production.yaml",
	"compose.prod.yml",
	"compose.prod.yaml",
	"compose.production.yml",
	"compose.production.yaml",
}

func (d *DockerComposeSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if !entry.IsDir() {
		for _, filename := range composeFiles {
			if strings.EqualFold(entry.Name(), filename) {
				composePath := d.filesystem.Join(rootPath, entry.Name())
				d.composeFiles = append(d.composeFiles, composePath)
				d.composeDirs[composePath] = rootPath
				break // Only take the first match per priority
			}
		}
	}

	return nil
}

func (d *DockerComposeSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(d.composeFiles) == 0 {
		return nil, nil
	}

	// Process the first compose file found (highest priority)
	composePath := d.composeFiles[0]
	workingDir := d.composeDirs[composePath]

	// Read compose file content through filesystem
	content, err := d.filesystem.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file %s: %w", composePath, err)
	}

	configDetails := composeTypes.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []composeTypes.ConfigFile{
			{
				Filename: composePath,
				Content:  content,
			},
		},
	}

	project, err := loader.LoadWithContext(ctx, configDetails, func(options *loader.Options) {
		options.SetProjectName(d.filesystem.Base(workingDir), true)
	})
	if err != nil {
		return nil, err
	}

	var services []types.Service
	for name, composeService := range project.Services {
		service := types.Service{
			Name:    name,
			Network: determineNetwork(composeService),
			Runtime: determineRuntime(composeService),
			Build:   determineBuild(composeService),
			Configs: []types.ConfigRef{
				{Type: "docker-compose", Path: composePath},
			},
		}

		// Set build path or image
		if service.Build == types.BuildFromSource && composeService.Build != nil {
			// Build context is relative to the compose file directory
			if composeService.Build.Context == "." {
				service.BuildPath = workingDir
			} else {
				service.BuildPath = d.filesystem.Join(workingDir, composeService.Build.Context)
			}
		} else if service.Build == types.BuildFromImage {
			service.Image = composeService.Image
		}

		services = append(services, service)
	}

	return services, nil
}

func determineNetwork(service composeTypes.ServiceConfig) types.Network {
	// No ports at all = background worker
	if len(service.Ports) == 0 && len(service.Expose) == 0 {
		return types.NetworkNone
	}

	// Check for explicit web indicators
	if hasWebIndicators(service) {
		return types.NetworkPublic
	}

	// Default: assume internal service (secure by default)
	return types.NetworkPrivate
}

func hasWebIndicators(service composeTypes.ServiceConfig) bool {
	// Published to standard web ports (80, 443)
	for _, port := range service.Ports {
		if port.Published == "" {
			continue
		}
		// Handle "0.0.0.0:80" format by taking last part after ":"
		parts := strings.Split(port.Published, ":")
		publishedPort := parts[len(parts)-1]

		if publishedPort == "80" || publishedPort == "443" {
			return true
		}
	}

	return false
}

func determineRuntime(service composeTypes.ServiceConfig) types.Runtime {
	// For now, assume all docker-compose services are continuous
	// TODO: Could check for restart policies or other indicators
	return types.RuntimeContinuous
}

func determineBuild(service composeTypes.ServiceConfig) types.Build {
	// Has build context = build from source
	if service.Build != nil {
		return types.BuildFromSource
	}

	// Uses pre-built image
	return types.BuildFromImage
}
