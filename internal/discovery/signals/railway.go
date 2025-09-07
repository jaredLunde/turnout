package signals

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/filesystems"
)

type RailwaySignal struct {
	filesystem  filesystems.FileSystem
	configPaths []string          // all found railway config files
	configDirs  map[string]string // config path -> directory path
}

func NewRailwaySignal(filesystem filesystems.FileSystem) *RailwaySignal {
	return &RailwaySignal{filesystem: filesystem}
}

func (r *RailwaySignal) Confidence() int {
	return 95 // Highest confidence - Railway configs are explicit production deployment specs
}

func (r *RailwaySignal) Reset() {
	r.configPaths = nil
	r.configDirs = make(map[string]string)
}

func (r *RailwaySignal) ObserveEntry(ctx context.Context, rootPath string, entry filesystems.DirEntry) error {
	if !entry.IsDir() {
		// Check for railway.json first (higher precedence), then railway.toml
		if strings.EqualFold(entry.Name(), "railway.json") {
			configPath := r.filesystem.Join(rootPath, entry.Name())
			r.configPaths = append(r.configPaths, configPath)
			r.configDirs[configPath] = rootPath
		} else if strings.EqualFold(entry.Name(), "railway.toml") && len(r.configPaths) == 0 {
			// Only set if we haven't already found railway.json
			configPath := r.filesystem.Join(rootPath, entry.Name())
			r.configPaths = append(r.configPaths, configPath)
			r.configDirs[configPath] = rootPath
		}
	}

	return nil
}

func (r *RailwaySignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(r.configPaths) == 0 {
		return nil, nil
	}

	// Use first config found
	configPath := r.configPaths[0]
	config, err := r.parseRailwayConfig(configPath)
	if err != nil {
		return nil, err
	}

	buildPath := r.configDirs[configPath]
	// Railway config defines a single service (unlike compose which can have multiple)
	service := types.Service{
		Name:      r.inferServiceNameFromPath(buildPath),
		Network:   determineNetworkFromRailway(config),
		Runtime:   types.RuntimeContinuous, // Railway services are continuous
		Build:     determineBuildFromRailway(config),
		BuildPath: buildPath, // Railway builds from the directory containing the config
		Configs: []types.ConfigRef{
			{Type: "railway", Path: configPath},
		},
	}

	return []types.Service{service}, nil
}

// RailwayConfig represents the Railway config-as-code schema
type RailwayConfig struct {
	Build  *RailwayBuild  `json:"build,omitempty" toml:"build,omitempty"`
	Deploy *RailwayDeploy `json:"deploy,omitempty" toml:"deploy,omitempty"`
}

type RailwayBuild struct {
	Builder      string `json:"builder,omitempty" toml:"builder,omitempty"`
	BuildCommand string `json:"buildCommand,omitempty" toml:"buildCommand,omitempty"`
}

type RailwayDeploy struct {
	StartCommand      string `json:"startCommand,omitempty" toml:"startCommand,omitempty"`
	HealthcheckPath   string `json:"healthcheckPath,omitempty" toml:"healthcheckPath,omitempty"`
	RestartPolicyType string `json:"restartPolicyType,omitempty" toml:"restartPolicyType,omitempty"`
}

func (r *RailwaySignal) parseRailwayConfig(configPath string) (*RailwayConfig, error) {
	data, err := r.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config RailwayConfig

	// Use the path extension to determine format
	if strings.HasSuffix(configPath, ".json") {
		err = json.Unmarshal(data, &config)
	} else {
		err = toml.Unmarshal(data, &config)
	}

	if err != nil {
		return nil, err
	}

	return &config, nil
}

func determineNetworkFromRailway(config *RailwayConfig) types.Network {
	// If there's a health check path, it's likely web-facing
	if config.Deploy != nil && config.Deploy.HealthcheckPath != "" {
		return types.NetworkPublic
	}

	// If there's a start command, assume it's a web service (Railway's primary use case)
	if config.Deploy != nil && config.Deploy.StartCommand != "" {
		return types.NetworkPublic
	}

	// Conservative default
	return types.NetworkPrivate
}

func determineBuildFromRailway(config *RailwayConfig) types.Build {
	// Railway services are built from source (that's the primary use case)
	return types.BuildFromSource
}

func (r *RailwaySignal) inferServiceNameFromPath(rootPath string) string {
	// Use directory name as service name
	return r.filesystem.Base(rootPath)
}
