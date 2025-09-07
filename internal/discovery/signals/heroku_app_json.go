package signals

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type HerokuAppJsonSignal struct {
	filesystem  fs.FileSystem
	configPaths []string          // all found app.json files
	configDirs  map[string]string // config path -> directory path
}

func NewHerokuAppJsonSignal(filesystem fs.FileSystem) *HerokuAppJsonSignal {
	return &HerokuAppJsonSignal{filesystem: filesystem}
}

func (h *HerokuAppJsonSignal) Confidence() int {
	return 90 // High confidence - app.json defines explicit app configuration
}

func (h *HerokuAppJsonSignal) Reset() {
	h.configPaths = nil
	h.configDirs = make(map[string]string)
}

func (h *HerokuAppJsonSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if !entry.IsDir() && strings.EqualFold(entry.Name(), "app.json") {
		configPath := h.filesystem.Join(rootPath, entry.Name())
		h.configPaths = append(h.configPaths, configPath)
		h.configDirs[configPath] = rootPath
	}

	return nil
}

func (h *HerokuAppJsonSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(h.configPaths) == 0 {
		return nil, nil
	}

	configPath := h.configPaths[0]
	config, err := h.parseAppJson(configPath)
	if err != nil {
		return nil, err
	}

	var services []types.Service

	buildPath := h.configDirs[configPath]
	// app.json typically describes one app, but we model it as a service
	serviceName := config.Name
	if serviceName == "" {
		serviceName = h.filesystem.Base(buildPath)
	}

	service := types.Service{
		Name:      serviceName,
		Network:   types.NetworkPublic,     // Heroku apps are typically web-facing
		Runtime:   types.RuntimeContinuous, // Apps run continuously
		Build:     types.BuildFromSource,   // Heroku builds from source
		BuildPath: buildPath,
		Configs: []types.ConfigRef{
			{Type: "heroku-app-json", Path: configPath},
		},
	}
	services = append(services, service)

	// Process addons as additional services
	for _, addon := range config.Addons {
		addonName := ""
		addonImage := ""

		switch a := addon.(type) {
		case string:
			// Simple addon name like "heroku-postgresql"
			addonName = a
			addonImage = inferImageFromAddon(a)
		case map[string]interface{}:
			// Addon object with plan details
			if plan, ok := a["plan"].(string); ok {
				addonName = plan
				addonImage = inferImageFromAddon(plan)
			}
		}

		if addonName != "" && addonImage != "" {
			addonService := types.Service{
				Name:    addonName,
				Network: types.NetworkPrivate, // Addons are typically private
				Runtime: types.RuntimeContinuous,
				Build:   types.BuildFromImage,
				Image:   addonImage,
				Configs: []types.ConfigRef{
					{Type: "heroku-app-json", Path: configPath},
				},
			}
			services = append(services, addonService)
		}
	}

	return services, nil
}

// HerokuAppJson represents the app.json configuration structure
type HerokuAppJson struct {
	Name         string                     `json:"name"`
	Description  string                     `json:"description"`
	Repository   string                     `json:"repository"`
	Logo         string                     `json:"logo"`
	Keywords     []string                   `json:"keywords"`
	Website      string                     `json:"website"`
	Stack        string                     `json:"stack"`
	Buildpacks   []HerokuBuildpack          `json:"buildpacks"`
	Env          map[string]HerokuEnvVar    `json:"env"`
	Formation    map[string]HerokuFormation `json:"formation"`
	Addons       []interface{}              `json:"addons"` // Can be string or object
	Scripts      *HerokuScripts             `json:"scripts"`
	Environments map[string]interface{}     `json:"environments"`
}

type HerokuBuildpack struct {
	URL string `json:"url"`
}

type HerokuEnvVar struct {
	Description string      `json:"description"`
	Value       interface{} `json:"value"`
	Required    bool        `json:"required"`
	Generator   string      `json:"generator"`
}

type HerokuFormation struct {
	Quantity int    `json:"quantity"`
	Size     string `json:"size"`
}

type HerokuScripts struct {
	PostDeploy string `json:"postdeploy"`
	PrDestroy  string `json:"pr-predestroy"`
}

func (h *HerokuAppJsonSignal) parseAppJson(configPath string) (*HerokuAppJson, error) {
	data, err := h.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config HerokuAppJson
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func inferImageFromAddon(addonName string) string {
	// Map common Heroku addon names to Docker images
	addonImageMap := map[string]string{
		"heroku-postgresql": "postgres",
		"postgresql":        "postgres",
		"postgres":          "postgres",
		"heroku-redis":      "redis",
		"redis":             "redis",
		"rediscloud":        "redis",
		"memcachier":        "memcached",
		"memcached":         "memcached",
		"mongodb":           "mongo",
		"mongolab":          "mongo",
		"mongohq":           "mongo",
	}

	// Try exact match first
	if image, exists := addonImageMap[addonName]; exists {
		return image
	}

	// Try partial matches for addon plans like "heroku-postgresql:hobby-dev"
	for addon, image := range addonImageMap {
		if strings.HasPrefix(addonName, addon) {
			return image
		}
	}

	return ""
}
