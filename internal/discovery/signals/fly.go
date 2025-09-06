package signals

import (
	"context"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type FlySignal struct {
	filesystem  fs.FileSystem
	configPaths []string              // all found fly.toml files
	configDirs  map[string]string     // config path -> directory path
}

func NewFlySignal(filesystem fs.FileSystem) *FlySignal {
	return &FlySignal{filesystem: filesystem}
}

func (f *FlySignal) Confidence() int {
	return 95 // Highest confidence - Fly configs are explicit production deployment specs
}

func (f *FlySignal) Reset() {
	f.configPaths = nil
	f.configDirs = make(map[string]string)
}

func (f *FlySignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if !entry.IsDir() && strings.EqualFold(entry.Name(), "fly.toml") {
		configPath := f.filesystem.Join(rootPath, entry.Name())
		f.configPaths = append(f.configPaths, configPath)
		f.configDirs[configPath] = rootPath
	}
	
	return nil
}

func (f *FlySignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(f.configPaths) == 0 {
		return nil, nil
	}

	var services []types.Service
	for _, configPath := range f.configPaths {
		config, err := f.parseFlyConfig(configPath)
		if err != nil {
			continue // Skip broken configs
		}

		buildPath := f.configDirs[configPath]
		// Fly.io apps are typically single-service (like Railway)
		service := types.Service{
			Name:      f.filesystem.Base(buildPath), // Use directory name for consistency
			Network:   determineNetworkFromFly(config),
			Runtime:   types.RuntimeContinuous, // Fly services are continuous
			Build:     determineBuildFromFly(config),
			BuildPath: buildPath, // Fly builds from the directory containing fly.toml
			Configs: []types.ConfigRef{
				{Type: "fly", Path: configPath},
			},
		}
		services = append(services, service)
	}

	return services, nil
}

// FlyConfig represents the fly.toml configuration structure
type FlyConfig struct {
	App           string            `toml:"app"`
	PrimaryRegion string            `toml:"primary_region,omitempty"`
	Build         *FlyBuild         `toml:"build,omitempty"`
	Deploy        *FlyDeploy        `toml:"deploy,omitempty"`
	Env           map[string]string `toml:"env,omitempty"`
	Services      []FlyService      `toml:"services,omitempty"`
	HTTPService   *FlyHTTPService   `toml:"http_service,omitempty"`
	VM            []FlyVM           `toml:"vm,omitempty"`
}

type FlyBuild struct {
	Image      string            `toml:"image,omitempty"`
	Dockerfile string            `toml:"dockerfile,omitempty"`
	Args       map[string]string `toml:"args,omitempty"`
}

type FlyDeploy struct {
	ReleaseCommand string `toml:"release_command,omitempty"`
	Strategy       string `toml:"strategy,omitempty"`
}

type FlyService struct {
	InternalPort int    `toml:"internal_port"`
	Protocol     string `toml:"protocol,omitempty"`
}

type FlyHTTPService struct {
	InternalPort       int      `toml:"internal_port"`
	ForceHTTPS         bool     `toml:"force_https,omitempty"`
	AutoStopMachines   bool     `toml:"auto_stop_machines,omitempty"`
	AutoStartMachines  bool     `toml:"auto_start_machines,omitempty"`
	MinMachinesRunning int      `toml:"min_machines_running,omitempty"`
	Processes          []string `toml:"processes,omitempty"`
}

type FlyVM struct {
	CPUKind  string `toml:"cpu_kind,omitempty"`
	CPUs     int    `toml:"cpus,omitempty"`
	MemoryMB int    `toml:"memory_mb,omitempty"`
}

func (f *FlySignal) parseFlyConfig(configPath string) (*FlyConfig, error) {
	var config FlyConfig
	content, err := f.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	_, err = toml.Decode(string(content), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func determineNetworkFromFly(config *FlyConfig) types.Network {
	// If there's an http_service, it's definitely public
	if config.HTTPService != nil {
		return types.NetworkPublic
	}

	// If there are services configured, likely public
	if len(config.Services) > 0 {
		return types.NetworkPublic
	}

	// Conservative default
	return types.NetworkPrivate
}

func determineBuildFromFly(config *FlyConfig) types.Build {
	// If there's a pre-built image specified, use that
	if config.Build != nil && config.Build.Image != "" {
		return types.BuildFromImage
	}

	// Otherwise, assume build from source (Fly's common use case)
	return types.BuildFromSource
}
