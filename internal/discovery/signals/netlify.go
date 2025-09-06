package signals

import (
	"context"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type NetlifySignal struct {
	filesystem      fs.FileSystem
	currentRootPath string
	configPaths     []string
}

func NewNetlifySignal(filesystem fs.FileSystem) *NetlifySignal {
	return &NetlifySignal{filesystem: filesystem}
}

func (n *NetlifySignal) Confidence() int {
	return 95 // Highest confidence - Netlify configs are explicit production deployment specs
}

func (n *NetlifySignal) Reset() {
	n.configPaths = nil
	n.currentRootPath = ""
}

func (n *NetlifySignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	n.currentRootPath = rootPath
	
	if !entry.IsDir() && strings.EqualFold(entry.Name(), "netlify.toml") {
		configPath := n.filesystem.Join(rootPath, entry.Name())
		n.configPaths = append(n.configPaths, configPath)
	}
	
	return nil
}

func (n *NetlifySignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(n.configPaths) == 0 {
		return nil, nil
	}

	configPath := n.configPaths[0]
	_, err := n.parseNetlifyConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Netlify deploys are typically single-service static sites
	service := types.Service{
		Name:      n.filesystem.Base(n.currentRootPath),
		Network:   types.NetworkPublic,     // Static sites are web-facing
		Runtime:   types.RuntimeContinuous, // CDN serves continuously
		Build:     types.BuildFromSource,   // Netlify builds from source
		BuildPath: n.currentRootPath,
		Configs: []types.ConfigRef{
			{Type: "netlify", Path: configPath},
		},
	}

	return []types.Service{service}, nil
}

// NetlifyConfig represents the netlify.toml configuration structure
type NetlifyConfig struct {
	Build     *NetlifyBuild       `toml:"build,omitempty"`
	Deploy    *NetlifyDeploy      `toml:"deploy,omitempty"`
	Context   map[string]*NetlifyBuild `toml:"context,omitempty"`
	Headers   []NetlifyHeaders    `toml:"headers,omitempty"`
	Redirects []NetlifyRedirects  `toml:"redirects,omitempty"`
	Edge      *NetlifyEdge        `toml:"edge,omitempty"`
	Template  *NetlifyTemplate    `toml:"template,omitempty"`
}

type NetlifyBuild struct {
	Base            string            `toml:"base,omitempty"`
	Command         string            `toml:"command,omitempty"`
	Publish         string            `toml:"publish,omitempty"`
	Functions       string            `toml:"functions,omitempty"`
	EdgeFunctions   string            `toml:"edge_functions,omitempty"`
	Environment     map[string]string `toml:"environment,omitempty"`
	ProcessingSkip  bool              `toml:"processing.skip,omitempty"`
	ProcessingCSS   map[string]bool   `toml:"processing.css,omitempty"`
	ProcessingJS    map[string]bool   `toml:"processing.js,omitempty"`
	ProcessingImages map[string]bool  `toml:"processing.images,omitempty"`
	ProcessingHTML  map[string]bool   `toml:"processing.html,omitempty"`
}

type NetlifyDeploy struct {
	Publish          string `toml:"publish,omitempty"`
	Production       bool   `toml:"production,omitempty"`
	PreviewBranch    string `toml:"preview_branch,omitempty"`
	SplitTestBranch  string `toml:"split_test_branch,omitempty"`
	AutoPublish      bool   `toml:"auto_publish,omitempty"`
}

type NetlifyHeaders struct {
	For    string            `toml:"for"`
	Values map[string]string `toml:"values"`
}

type NetlifyRedirects struct {
	From       string `toml:"from"`
	To         string `toml:"to"`
	Status     int    `toml:"status,omitempty"`
	Force      bool   `toml:"force,omitempty"`
	Query      string `toml:"query,omitempty"`
	Conditions string `toml:"conditions,omitempty"`
	Headers    string `toml:"headers,omitempty"`
	Signed     string `toml:"signed,omitempty"`
}

type NetlifyEdge struct {
	Functions []NetlifyEdgeFunction `toml:"functions,omitempty"`
}

type NetlifyEdgeFunction struct {
	Function string   `toml:"function"`
	Path     []string `toml:"path"`
}

type NetlifyContext struct {
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