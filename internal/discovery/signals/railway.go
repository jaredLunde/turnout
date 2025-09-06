package signals

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type RailwaySignal struct{
	filesystem fs.FileSystem
}

func NewRailwaySignal(filesystem fs.FileSystem) *RailwaySignal {
	return &RailwaySignal{filesystem: filesystem}
}

func (r *RailwaySignal) Confidence() int {
	return 95 // Highest confidence - Railway configs are explicit production deployment specs
}

func (r *RailwaySignal) Discover(ctx context.Context, rootPath string, dirEntries []fs.DirEntry) ([]types.Service, error) {
	// Look for Railway config files
	configPath, err := r.findRailwayConfig(rootPath, dirEntries)
	if err != nil || configPath == "" {
		return nil, err
	}

	config, err := r.parseRailwayConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Railway config defines a single service (unlike compose which can have multiple)
	service := types.Service{
		Name:      r.inferServiceNameFromPath(rootPath),
		Network:   determineNetworkFromRailway(config),
		Runtime:   types.RuntimeContinuous, // Railway services are continuous
		Build:     determineBuildFromRailway(config),
		BuildPath: rootPath, // Railway builds from the directory containing the config
		Configs: []types.ConfigRef{
			{Type: "railway", Path: configPath},
		},
	}

	return []types.Service{service}, nil
}

func (r *RailwaySignal) findRailwayConfig(rootPath string, dirEntries []fs.DirEntry) (string, error) {
	// Check for railway.json first, then railway.toml
	if found, err := fs.FindFileInEntries(r.filesystem, rootPath, "railway.json", dirEntries); err == nil && found != "" {
		return found, nil
	}
	
	if found, err := fs.FindFileInEntries(r.filesystem, rootPath, "railway.toml", dirEntries); err == nil && found != "" {
		return found, nil
	}

	return "", nil
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