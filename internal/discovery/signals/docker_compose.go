package signals

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/railwayapp/turnout/internal/discovery/types"
)

type DockerComposeSignal struct{}

func (d *DockerComposeSignal) Confidence() int {
	return 90 // Very high confidence - docker-compose explicitly defines services
}

func (d *DockerComposeSignal) Discover(ctx context.Context, rootPath string) ([]types.Service, error) {
	// Try common compose file names
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml", 
		"compose.yml",
		"compose.yaml",
	}
	
	var composePath string
	for _, filename := range composeFiles {
		path := filepath.Join(rootPath, filename)
		if _, err := os.Stat(path); err == nil {
			composePath = path
			break
		}
	}
	
	if composePath == "" {
		return nil, os.ErrNotExist
	}
	
	configDetails := composeTypes.ConfigDetails{
		WorkingDir: rootPath,
		ConfigFiles: []composeTypes.ConfigFile{
			{Filename: composePath},
		},
	}
	
	project, err := loader.LoadWithContext(ctx, configDetails, func(options *loader.Options) {
		options.SetProjectName(filepath.Base(rootPath), true)
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
			service.BuildPath = composeService.Build.Context
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