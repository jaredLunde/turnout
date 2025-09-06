package signals

import (
	"context"

	"github.com/BurntSushi/toml"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type NetlifySignal struct {
	filesystem fs.FileSystem
}

func NewNetlifySignal(filesystem fs.FileSystem) *NetlifySignal {
	return &NetlifySignal{filesystem: filesystem}
}

func (n *NetlifySignal) Confidence() int {
	return 95 // Highest confidence - Netlify configs are explicit production deployment specs
}

func (n *NetlifySignal) Discover(ctx context.Context, rootPath string, dirEntries []fs.DirEntry) ([]types.Service, error) {
	// Look for netlify.toml
	configPath, err := fs.FindFileInEntries(n.filesystem, rootPath, "netlify.toml", dirEntries)
	if err != nil || configPath == "" {
		return nil, err
	}

	_, err = n.parseNetlifyConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Netlify deployments are single-service (static site + edge functions)
	service := types.Service{
		Name:      n.filesystem.Base(rootPath),
		Network:   types.NetworkPublic,     // Netlify deployments are web-facing
		Runtime:   types.RuntimeContinuous, // Web deployments run continuously
		Build:     types.BuildFromSource,   // Netlify builds from source
		BuildPath: rootPath,
		Configs: []types.ConfigRef{
			{Type: "netlify", Path: configPath},
		},
	}

	return []types.Service{service}, nil
}

// NetlifyConfig represents the netlify.toml configuration structure
type NetlifyConfig struct {
	Build     *NetlifyBuild             `toml:"build"`
	Context   map[string]NetlifyContext `toml:"context"`
	Functions *NetlifyFunctions         `toml:"functions"`
	Redirects []NetlifyRedirect         `toml:"redirects"`
	Headers   []NetlifyHeader           `toml:"headers"`
	Template  *NetlifyTemplate          `toml:"template"`
}

type NetlifyBuild struct {
	Publish   string `toml:"publish"`
	Command   string `toml:"command"`
	Functions string `toml:"functions"`
	Base      string `toml:"base"`
}

type NetlifyContext struct {
	Publish     string            `toml:"publish"`
	Command     string            `toml:"command"`
	Functions   string            `toml:"functions"`
	Base        string            `toml:"base"`
	Environment map[string]string `toml:"environment"`
}

type NetlifyFunctions struct {
	Directory string `toml:"directory"`
}

type NetlifyRedirect struct {
	From   string `toml:"from"`
	To     string `toml:"to"`
	Status int    `toml:"status"`
}

type NetlifyHeader struct {
	For    string            `toml:"for"`
	Values map[string]string `toml:"values"`
}

type NetlifyTemplate struct {
	Incoming []string `toml:"incoming"`
}

func (n *NetlifySignal) parseNetlifyConfig(configPath string) (*NetlifyConfig, error) {
	var config NetlifyConfig
	content, err := n.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	_, err = toml.Decode(string(content), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
