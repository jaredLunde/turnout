package signals

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type HerokuAppJsonSignal struct{}

func (h *HerokuAppJsonSignal) Confidence() int {
	return 90 // High confidence - app.json defines explicit app configuration
}

func (h *HerokuAppJsonSignal) Discover(ctx context.Context, rootPath string) ([]types.Service, error) {
	// Look for app.json
	configPath, err := fs.FindFile(rootPath, "app.json")
	if err != nil || configPath == "" {
		return nil, err
	}

	config, err := parseAppJson(configPath)
	if err != nil {
		return nil, err
	}

	var services []types.Service

	// app.json typically describes one app, but we model it as a service
	serviceName := config.Name
	if serviceName == "" {
		serviceName = inferServiceNameFromPath(rootPath)
	}

	service := types.Service{
		Name:      serviceName,
		Network:   types.NetworkPublic, // Heroku apps are typically web-facing
		Runtime:   types.RuntimeContinuous, // Apps run continuously
		Build:     types.BuildFromSource, // Heroku builds from source
		BuildPath: rootPath,
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
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Repository  string                     `json:"repository"`
	Logo        string                     `json:"logo"`
	Keywords    []string                   `json:"keywords"`
	Website     string                     `json:"website"`
	Stack       string                     `json:"stack"`
	Buildpacks  []HerokuBuildpack          `json:"buildpacks"`
	Env         map[string]HerokuEnvVar    `json:"env"`
	Formation   map[string]HerokuFormation `json:"formation"`
	Addons      []interface{}              `json:"addons"` // Can be string or object
	Scripts     *HerokuScripts             `json:"scripts"`
	Environments map[string]interface{}    `json:"environments"`
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

func parseAppJson(configPath string) (*HerokuAppJson, error) {
	data, err := os.ReadFile(configPath)
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