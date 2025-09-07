package signals

import (
	"context"
	"strings"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/filesystems"
	"gopkg.in/yaml.v3"
)

type SkaffoldSignal struct {
	filesystem  filesystems.FileSystem
	configPaths []string          // all found skaffold.yaml files
	configDirs  map[string]string // config path -> directory path
}

func NewSkaffoldSignal(filesystem filesystems.FileSystem) *SkaffoldSignal {
	return &SkaffoldSignal{filesystem: filesystem}
}

func (s *SkaffoldSignal) Confidence() int {
	return 95 // Very high confidence - Skaffold configs are explicit deployment specs
}

func (s *SkaffoldSignal) Reset() {
	s.configPaths = nil
	s.configDirs = make(map[string]string)
}

func (s *SkaffoldSignal) ObserveEntry(ctx context.Context, rootPath string, entry filesystems.DirEntry) error {
	if !entry.IsDir() && strings.EqualFold(entry.Name(), "skaffold.yaml") {
		configPath := s.filesystem.Join(rootPath, entry.Name())
		s.configPaths = append(s.configPaths, configPath)
		s.configDirs[configPath] = rootPath
	}

	return nil
}

func (s *SkaffoldSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(s.configPaths) == 0 {
		return nil, nil
	}

	var services []types.Service
	for _, configPath := range s.configPaths {
		config, err := s.parseSkaffoldConfig(configPath)
		if err != nil {
			continue // Skip broken configs
		}

		buildPath := s.configDirs[configPath]

		// Skaffold can define multiple services in one config
		if len(config.Build.Artifacts) > 0 {
			for _, artifact := range config.Build.Artifacts {
				service := types.Service{
					Name:      s.deriveServiceName(artifact.ImageName, buildPath),
					Network:   s.determineNetworkFromSkaffold(config),
					Runtime:   types.RuntimeContinuous, // Skaffold services are typically continuous
					Build:     s.determineBuildFromSkaffold(artifact),
					BuildPath: s.determineBuildPath(artifact, buildPath),
					Configs: []types.ConfigRef{
						{Type: "skaffold", Path: configPath},
					},
				}

				// Set image if using pre-built image
				if service.Build == types.BuildFromImage {
					service.Image = artifact.ImageName
				}

				services = append(services, service)
			}
		} else {
			// Single service fallback
			service := types.Service{
				Name:      s.filesystem.Base(buildPath),
				Network:   types.NetworkPrivate, // Conservative default
				Runtime:   types.RuntimeContinuous,
				Build:     types.BuildFromSource, // Fallback assumes build from source
				BuildPath: buildPath,
				Configs: []types.ConfigRef{
					{Type: "skaffold", Path: configPath},
				},
			}
			services = append(services, service)
		}
	}

	return services, nil
}

func (s *SkaffoldSignal) parseSkaffoldConfig(configPath string) (*latest.SkaffoldConfig, error) {
	content, err := s.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config latest.SkaffoldConfig
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *SkaffoldSignal) deriveServiceName(imageName, buildPath string) string {
	if imageName != "" {
		// Extract service name from image name (e.g., "myapp" from "gcr.io/project/myapp")
		parts := strings.Split(imageName, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return s.filesystem.Base(buildPath)
}

func (s *SkaffoldSignal) determineNetworkFromSkaffold(config *latest.SkaffoldConfig) types.Network {
	// Skaffold deploys to Kubernetes, but we can't easily determine if services are public
	// without parsing the Kubernetes manifests. Conservative default.
	return types.NetworkPrivate
}

func (s *SkaffoldSignal) determineBuildFromSkaffold(artifact *latest.Artifact) types.Build {
	// If artifact has Docker build context, it builds from source
	if artifact.DockerArtifact != nil {
		return types.BuildFromSource
	}

	// If artifact has Jib (Java) build, it builds from source
	if artifact.JibArtifact != nil {
		return types.BuildFromSource
	}

	// If artifact has Bazel build, it builds from source
	if artifact.BazelArtifact != nil {
		return types.BuildFromSource
	}

	// If artifact has Ko (Go) build, it builds from source
	if artifact.KoArtifact != nil {
		return types.BuildFromSource
	}

	// If artifact has custom build, assume it builds from source
	if artifact.CustomArtifact != nil {
		return types.BuildFromSource
	}

	// If no build artifact specified, assume pre-built image
	return types.BuildFromImage
}

func (s *SkaffoldSignal) determineBuildPath(artifact *latest.Artifact, configDir string) string {
	// Use the artifact's workspace (context) - this is the primary build directory
	if artifact.Workspace != "" {
		return s.resolveRelativePath(artifact.Workspace, configDir)
	}

	// Fall back to dockerfile directory if no explicit workspace
	if artifact.DockerArtifact != nil && artifact.DockerArtifact.DockerfilePath != "" {
		return s.resolveRelativePath(s.filesystem.Dir(artifact.DockerArtifact.DockerfilePath), configDir)
	}

	// Default to the skaffold.yaml directory only as last resort
	return configDir
}

func (s *SkaffoldSignal) resolveRelativePath(path, configDir string) string {
	if strings.HasPrefix(path, "./") {
		// Remove leading "./" and join with config directory
		return s.filesystem.Join(configDir, path[2:])
	}
	if path == "." {
		return configDir
	}
	// If not relative, assume it's already resolved
	return path
}
