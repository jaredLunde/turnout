package signals

import (
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
	"gopkg.in/yaml.v3"
)

type RenderSignal struct {
	filesystem  fs.FileSystem
	configPaths []string          // all found render.yaml files
	configDirs  map[string]string // config path -> directory path
}

func NewRenderSignal(filesystem fs.FileSystem) *RenderSignal {
	return &RenderSignal{filesystem: filesystem}
}

func (r *RenderSignal) Confidence() int {
	return 95 // Highest confidence - Render Blueprints are explicit production deployment specs
}

func (r *RenderSignal) Reset() {
	r.configPaths = nil
	r.configDirs = make(map[string]string)
}

func (r *RenderSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if !entry.IsDir() && strings.EqualFold(entry.Name(), "render.yaml") {
		configPath := r.filesystem.Join(rootPath, entry.Name())
		r.configPaths = append(r.configPaths, configPath)
		r.configDirs[configPath] = rootPath
	}

	return nil
}

func (r *RenderSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(r.configPaths) == 0 {
		return nil, nil
	}

	var allServices []types.Service

	for _, configPath := range r.configPaths {
		config, err := r.parseRenderConfig(configPath)
		if err != nil {
			continue // Skip broken configs
		}

		buildPath := r.configDirs[configPath]
		// Add regular services
		for _, renderService := range config.Services {
			service := types.Service{
				Name:      renderService.Name,
				Network:   determineNetworkFromRender(renderService),
				Runtime:   determineRuntimeFromRender(renderService),
				Build:     determineBuildFromRender(renderService),
				BuildPath: buildPath, // Render builds from repo root by default
				Configs: []types.ConfigRef{
					{Type: "render", Path: configPath},
				},
			}

			// Set image for prebuilt Docker images
			if renderService.Image != nil && renderService.Image.URL != "" {
				service.Image = renderService.Image.URL
			}

			allServices = append(allServices, service)
		}

		// Add databases as services
		for _, renderDB := range config.Databases {
			service := types.Service{
				Name:    renderDB.Name,
				Network: types.NetworkPrivate,    // Databases are typically private
				Runtime: types.RuntimeContinuous, // Databases run continuously
				Build:   types.BuildFromImage,    // Databases use pre-built images
				Image:   "postgres",              // Could be more specific based on version
				Configs: []types.ConfigRef{
					{Type: "render", Path: configPath},
				},
			}
			allServices = append(allServices, service)
		}
	}

	return allServices, nil
}

// RenderConfig represents the render.yaml blueprint structure
type RenderConfig struct {
	Services     []RenderService     `yaml:"services"`
	Databases    []RenderDatabase    `yaml:"databases,omitempty"`
	EnvVarGroups []RenderEnvVarGroup `yaml:"envVarGroups,omitempty"`
	Previews     *RenderPreviews     `yaml:"previews,omitempty"`
}

type RenderService struct {
	Name            string         `yaml:"name"`
	Type            string         `yaml:"type"`
	Runtime         string         `yaml:"runtime,omitempty"`
	Plan            string         `yaml:"plan,omitempty"`
	Region          string         `yaml:"region,omitempty"`
	Repo            string         `yaml:"repo,omitempty"`
	Branch          string         `yaml:"branch,omitempty"`
	BuildCommand    string         `yaml:"buildCommand,omitempty"`
	StartCommand    string         `yaml:"startCommand,omitempty"`
	Schedule        string         `yaml:"schedule,omitempty"`
	Domains         []string       `yaml:"domains,omitempty"`
	HealthCheckPath string         `yaml:"healthCheckPath,omitempty"`
	Image           *RenderImage   `yaml:"image,omitempty"`
	Scaling         *RenderScaling `yaml:"scaling,omitempty"`
	NumInstances    int            `yaml:"numInstances,omitempty"`
	EnvVars         []RenderEnvVar `yaml:"envVars,omitempty"`
}

type RenderImage struct {
	URL   string            `yaml:"url"`
	Creds *RenderImageCreds `yaml:"creds,omitempty"`
}

type RenderImageCreds struct {
	FromRegistryCreds *RenderRegistryCred `yaml:"fromRegistryCreds,omitempty"`
}

type RenderRegistryCred struct {
	Name string `yaml:"name"`
}

type RenderScaling struct {
	MinInstances        int `yaml:"minInstances"`
	MaxInstances        int `yaml:"maxInstances"`
	TargetCPUPercent    int `yaml:"targetCPUPercent,omitempty"`
	TargetMemoryPercent int `yaml:"targetMemoryPercent,omitempty"`
}

type RenderEnvVar struct {
	Key           string                `yaml:"key,omitempty"`
	Value         string                `yaml:"value,omitempty"`
	GenerateValue bool                  `yaml:"generateValue,omitempty"`
	Sync          bool                  `yaml:"sync,omitempty"`
	FromDatabase  *RenderEnvFromDB      `yaml:"fromDatabase,omitempty"`
	FromService   *RenderEnvFromService `yaml:"fromService,omitempty"`
	FromGroup     string                `yaml:"fromGroup,omitempty"`
}

type RenderEnvFromDB struct {
	Name     string `yaml:"name"`
	Property string `yaml:"property"`
}

type RenderEnvFromService struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`
	Property  string `yaml:"property,omitempty"`
	EnvVarKey string `yaml:"envVarKey,omitempty"`
}

type RenderDatabase struct {
	Name string `yaml:"name"`
	Plan string `yaml:"plan,omitempty"`
}

type RenderEnvVarGroup struct {
	Name    string         `yaml:"name"`
	EnvVars []RenderEnvVar `yaml:"envVars"`
}

type RenderPreviews struct {
	Generation string `yaml:"generation,omitempty"`
}

func (r *RenderSignal) parseRenderConfig(configPath string) (*RenderConfig, error) {
	data, err := r.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config RenderConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func determineNetworkFromRender(service RenderService) types.Network {
	// Web services with domains are definitely public
	if service.Type == "web" && len(service.Domains) > 0 {
		return types.NetworkPublic
	}

	// Web services with health checks are likely public
	if service.Type == "web" && service.HealthCheckPath != "" {
		return types.NetworkPublic
	}

	// Web services are generally public by default
	if service.Type == "web" {
		return types.NetworkPublic
	}

	// Private services are explicitly private
	if service.Type == "pserv" {
		return types.NetworkPrivate
	}

	// Workers, cron jobs, etc. are typically private
	return types.NetworkPrivate
}

func determineRuntimeFromRender(service RenderService) types.Runtime {
	// Cron jobs are scheduled
	if service.Type == "cron" || service.Schedule != "" {
		return types.RuntimeScheduled
	}

	// Everything else is continuous
	return types.RuntimeContinuous
}

func determineBuildFromRender(service RenderService) types.Build {
	// If there's a prebuilt image specified, use that
	if service.Image != nil && service.Image.URL != "" {
		return types.BuildFromImage
	}

	// Otherwise, assume build from source
	return types.BuildFromSource
}
