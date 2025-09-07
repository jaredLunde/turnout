package signals

import (
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
	"gopkg.in/yaml.v3"
)

type ServerlessSignal struct {
	filesystem  fs.FileSystem
	configPaths []string          // all found serverless.yml files
	configDirs  map[string]string // config path -> directory path
}

func NewServerlessSignal(filesystem fs.FileSystem) *ServerlessSignal {
	return &ServerlessSignal{filesystem: filesystem}
}

func (s *ServerlessSignal) Confidence() int {
	return 95 // Very high confidence - Serverless configs are explicit deployment specs
}

func (s *ServerlessSignal) Reset() {
	s.configPaths = nil
	s.configDirs = make(map[string]string)
}

func (s *ServerlessSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if !entry.IsDir() && matchesServerlessConfig(entry.Name()) {
		configPath := s.filesystem.Join(rootPath, entry.Name())
		s.configPaths = append(s.configPaths, configPath)
		s.configDirs[configPath] = rootPath
	}

	return nil
}

func (s *ServerlessSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(s.configPaths) == 0 {
		return nil, nil
	}

	var services []types.Service
	for _, configPath := range s.configPaths {
		config, err := s.parseServerlessConfig(configPath)
		if err != nil {
			continue // Skip broken configs
		}

		buildPath := s.configDirs[configPath]
		service := types.Service{
			Name:      s.deriveServiceName(config, buildPath),
			Network:   s.determineNetworkFromServerless(config),
			Runtime:   s.determineRuntimeFromServerless(config),
			Build:     s.determineBuildFromServerless(config),
			BuildPath: buildPath,
			Configs: []types.ConfigRef{
				{Type: "serverless", Path: configPath},
			},
		}

		// Set image if using pre-built container image
		if image := s.extractImageFromServerless(config); image != "" {
			service.Image = image
		}
		services = append(services, service)
	}

	return services, nil
}

// ServerlessConfig represents the basic serverless.yml structure
type ServerlessConfig struct {
	Service   string                 `yaml:"service"`
	Provider  ServerlessProvider     `yaml:"provider"`
	Functions map[string]interface{} `yaml:"functions,omitempty"`
	Resources interface{}            `yaml:"resources,omitempty"`
	Custom    map[string]interface{} `yaml:"custom,omitempty"`
}

type ServerlessProvider struct {
	Name    string `yaml:"name"`
	Runtime string `yaml:"runtime,omitempty"`
	Region  string `yaml:"region,omitempty"`
}

func (s *ServerlessSignal) parseServerlessConfig(configPath string) (*ServerlessConfig, error) {
	var config ServerlessConfig
	content, err := s.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *ServerlessSignal) deriveServiceName(config *ServerlessConfig, buildPath string) string {
	if config.Service != "" {
		return config.Service
	}
	return s.filesystem.Base(buildPath)
}

func (s *ServerlessSignal) determineNetworkFromServerless(config *ServerlessConfig) types.Network {
	// Check functions for HTTP event triggers (indicating public API endpoints)
	for _, function := range config.Functions {
		if funcMap, ok := function.(map[string]interface{}); ok {
			if events, exists := funcMap["events"]; exists {
				if eventList, ok := events.([]interface{}); ok {
					for _, event := range eventList {
						if eventMap, ok := event.(map[string]interface{}); ok {
							// HTTP events indicate public endpoints
							if _, hasHttp := eventMap["http"]; hasHttp {
								return types.NetworkPublic
							}
							if _, hasHttpApi := eventMap["httpApi"]; hasHttpApi {
								return types.NetworkPublic
							}
						}
					}
				}
			}
		}
	}

	// Conservative default - functions without HTTP events are likely private
	return types.NetworkPrivate
}

func (s *ServerlessSignal) determineRuntimeFromServerless(config *ServerlessConfig) types.Runtime {
	// Check functions for scheduled/cron event triggers
	for _, function := range config.Functions {
		if funcMap, ok := function.(map[string]interface{}); ok {
			if events, exists := funcMap["events"]; exists {
				if eventList, ok := events.([]interface{}); ok {
					for _, event := range eventList {
						if eventMap, ok := event.(map[string]interface{}); ok {
							// Schedule/cron events indicate scheduled runtime
							if _, hasSchedule := eventMap["schedule"]; hasSchedule {
								return types.RuntimeScheduled
							}
						}
					}
				}
			}
		}
	}

	// Most serverless functions are event-driven but run continuously
	return types.RuntimeContinuous
}

func (s *ServerlessSignal) determineBuildFromServerless(config *ServerlessConfig) types.Build {
	// Check if using container images (newer serverless feature)
	if image := s.extractImageFromServerless(config); image != "" {
		return types.BuildFromImage
	}

	// Most serverless deployments build from source (zip packages)
	return types.BuildFromSource
}

func (s *ServerlessSignal) extractImageFromServerless(config *ServerlessConfig) string {
	// Check provider-level image configuration
	if config.Provider.Name == "aws" {
		// Look for ECR image references in functions
		for _, function := range config.Functions {
			if funcMap, ok := function.(map[string]interface{}); ok {
				if image, exists := funcMap["image"]; exists {
					if imageStr, ok := image.(string); ok {
						return imageStr
					}
				}
			}
		}
	}

	// No container image found
	return ""
}

func matchesServerlessConfig(name string) bool {
	serverlessFiles := []string{
		"serverless.yml",
		"serverless.yaml",
		"serverless.json",
	}

	for _, pattern := range serverlessFiles {
		if strings.EqualFold(name, pattern) {
			return true
		}
	}
	return false
}
