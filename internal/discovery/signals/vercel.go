package signals

import (
	"context"
	"encoding/json"
	"iter"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type VercelSignal struct{
	filesystem fs.FileSystem
}

func NewVercelSignal(filesystem fs.FileSystem) *VercelSignal {
	return &VercelSignal{filesystem: filesystem}
}

func (v *VercelSignal) Confidence() int {
	return 95 // Highest confidence - Vercel configs are explicit production deployment specs
}

func (v *VercelSignal) Discover(ctx context.Context, rootPath string, dirEntries iter.Seq2[fs.DirEntry, error]) ([]types.Service, error) {
	// Look for vercel.json
	configPath, err := fs.FindFileInEntries(v.filesystem, rootPath, "vercel.json", dirEntries)
	if err != nil || configPath == "" {
		return nil, err
	}

	_, err = v.parseVercelConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Vercel deploys are typically single-service (static site + serverless functions)
	// but we model it as one service representing the deployment
	service := types.Service{
		Name:      v.filesystem.Base(rootPath),
		Network:   types.NetworkPublic, // Vercel deployments are web-facing
		Runtime:   types.RuntimeContinuous, // Web deployments run continuously
		Build:     types.BuildFromSource, // Vercel builds from source
		BuildPath: rootPath,
		Configs: []types.ConfigRef{
			{Type: "vercel", Path: configPath},
		},
	}

	return []types.Service{service}, nil
}

// VercelConfig represents the vercel.json configuration structure
type VercelConfig struct {
	Version     int                    `json:"version,omitempty"`
	Regions     []string              `json:"regions,omitempty"`
	Builds      []VercelBuild         `json:"builds,omitempty"` // Deprecated
	Functions   map[string]VercelFunc `json:"functions,omitempty"`
	Redirects   []VercelRedirect      `json:"redirects,omitempty"`
	Rewrites    []VercelRewrite       `json:"rewrites,omitempty"`
	Headers     []VercelHeader        `json:"headers,omitempty"`
	Env         map[string]string     `json:"env,omitempty"`
	Build       *VercelBuildConfig    `json:"build,omitempty"`
	Git         *VercelGit            `json:"git,omitempty"`
	CleanUrls   bool                  `json:"cleanUrls,omitempty"`
}

type VercelBuild struct {
	Src    string            `json:"src"`
	Use    string            `json:"use"`
	Config map[string]interface{} `json:"config,omitempty"`
}

type VercelFunc struct {
	Runtime    string `json:"runtime,omitempty"`
	Memory     int    `json:"memory,omitempty"`
	MaxDuration int   `json:"maxDuration,omitempty"`
}

type VercelRedirect struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Permanent   bool   `json:"permanent,omitempty"`
}

type VercelRewrite struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type VercelHeader struct {
	Source  string            `json:"source"`
	Headers []VercelHeaderKV  `json:"headers"`
}

type VercelHeaderKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type VercelBuildConfig struct {
	Env map[string]string `json:"env,omitempty"`
}

type VercelGit struct {
	DeploymentEnabled map[string]bool `json:"deploymentEnabled,omitempty"`
}

func (v *VercelSignal) parseVercelConfig(configPath string) (*VercelConfig, error) {
	data, err := v.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config VercelConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}