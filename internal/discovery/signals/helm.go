package signals

import (
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
	"helm.sh/helm/v3/pkg/chart"
	"gopkg.in/yaml.v3"
)

type HelmSignal struct {
	filesystem  fs.FileSystem
	configPaths []string          // all found Chart.yaml files
	configDirs  map[string]string // config path -> directory path
}

func NewHelmSignal(filesystem fs.FileSystem) *HelmSignal {
	return &HelmSignal{filesystem: filesystem}
}

func (h *HelmSignal) Confidence() int {
	return 95 // Very high confidence - Helm charts are explicit deployment specs
}

func (h *HelmSignal) Reset() {
	h.configPaths = nil
	h.configDirs = make(map[string]string)
}

func (h *HelmSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if !entry.IsDir() && strings.EqualFold(entry.Name(), "Chart.yaml") {
		configPath := h.filesystem.Join(rootPath, entry.Name())
		h.configPaths = append(h.configPaths, configPath)
		h.configDirs[configPath] = rootPath
	}

	return nil
}

func (h *HelmSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(h.configPaths) == 0 {
		return nil, nil
	}

	var services []types.Service
	for _, configPath := range h.configPaths {
		chart, err := h.parseChartYaml(configPath)
		if err != nil {
			// Create a basic service even if chart parsing fails
			buildPath := h.configDirs[configPath]
			service := types.Service{
				Name:      h.filesystem.Base(buildPath),
				Network:   types.NetworkPrivate,
				Runtime:   types.RuntimeContinuous,
				Build:     types.BuildFromImage,
				BuildPath: buildPath,
				Configs: []types.ConfigRef{
					{Type: "helm", Path: configPath},
				},
			}
			services = append(services, service)
			continue
		}

		buildPath := h.configDirs[configPath]
		
		service := types.Service{
			Name:      h.deriveServiceName(chart, buildPath),
			Network:   h.determineNetworkFromHelm(buildPath),
			Runtime:   types.RuntimeContinuous,
			Build:     h.determineBuildFromHelm(buildPath),
			BuildPath: buildPath,
			Configs: []types.ConfigRef{
				{Type: "helm", Path: configPath},
			},
		}

		// Set image if using pre-built image
		if service.Build == types.BuildFromImage {
			if image := h.extractImageFromHelm(buildPath); image != "" {
				service.Image = image
			}
		}

		services = append(services, service)
	}

	return services, nil
}

func (h *HelmSignal) deriveServiceName(chart *chart.Metadata, buildPath string) string {
	if chart.Name != "" {
		return chart.Name
	}
	return h.filesystem.Base(buildPath)
}

func (h *HelmSignal) determineNetworkFromHelm(chartDir string) types.Network {
	// Check templates for Ingress or LoadBalancer services
	if h.hasIngressOrLoadBalancer(chartDir) {
		return types.NetworkPublic
	}
	
	// Check values.yaml for ingress configuration
	if h.hasIngressInValues(chartDir) {
		return types.NetworkPublic
	}
	
	// Conservative default
	return types.NetworkPrivate
}

func (h *HelmSignal) determineBuildFromHelm(chartDir string) types.Build {
	// Check if values.yaml has image references
	if h.hasImageInValues(chartDir) {
		return types.BuildFromImage
	}
	
	// Check templates for image references
	if h.hasImageInTemplates(chartDir) {
		return types.BuildFromImage
	}
	
	// Default to source build if no clear image refs
	return types.BuildFromSource
}

func (h *HelmSignal) extractImageFromHelm(chartDir string) string {
	// Try to extract image from values.yaml
	return h.getImageFromValues(chartDir)
}

func (h *HelmSignal) hasIngressOrLoadBalancer(chartDir string) bool {
	templatesDir := h.filesystem.Join(chartDir, "templates")
	
	// Read all template files using iterator
	for entry, err := range h.filesystem.ReadDir(templatesDir) {
		if err != nil {
			continue
		}
		
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		
		templatePath := h.filesystem.Join(templatesDir, entry.Name())
		content, err := h.filesystem.ReadFile(templatePath)
		if err != nil {
			continue
		}
		
		contentStr := string(content)
		if strings.Contains(contentStr, "kind: Ingress") || 
		   strings.Contains(contentStr, "type: LoadBalancer") {
			return true
		}
	}
	
	return false
}

func (h *HelmSignal) hasIngressInValues(chartDir string) bool {
	valuesPath := h.filesystem.Join(chartDir, "values.yaml")
	content, err := h.filesystem.ReadFile(valuesPath)
	if err != nil {
		return false
	}
	
	var values map[string]interface{}
	if err := yaml.Unmarshal(content, &values); err != nil {
		return false
	}
	
	// Check for ingress.enabled or similar patterns
	if ingress, ok := values["ingress"].(map[string]interface{}); ok {
		if enabled, ok := ingress["enabled"].(bool); ok && enabled {
			return true
		}
	}
	
	return false
}

func (h *HelmSignal) hasImageInValues(chartDir string) bool {
	valuesPath := h.filesystem.Join(chartDir, "values.yaml")
	content, err := h.filesystem.ReadFile(valuesPath)
	if err != nil {
		return false
	}
	
	var values map[string]interface{}
	if err := yaml.Unmarshal(content, &values); err != nil {
		return false
	}
	
	return h.containsImageConfig(values)
}

func (h *HelmSignal) hasImageInTemplates(chartDir string) bool {
	templatesDir := h.filesystem.Join(chartDir, "templates")
	
	for entry, err := range h.filesystem.ReadDir(templatesDir) {
		if err != nil {
			continue
		}
		
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		
		templatePath := h.filesystem.Join(templatesDir, entry.Name())
		content, err := h.filesystem.ReadFile(templatePath)
		if err != nil {
			continue
		}
		
		contentStr := string(content)
		if strings.Contains(contentStr, "image:") || strings.Contains(contentStr, ".image.") {
			return true
		}
	}
	
	return false
}

func (h *HelmSignal) getImageFromValues(chartDir string) string {
	valuesPath := h.filesystem.Join(chartDir, "values.yaml")
	content, err := h.filesystem.ReadFile(valuesPath)
	if err != nil {
		return ""
	}
	
	var values map[string]interface{}
	if err := yaml.Unmarshal(content, &values); err != nil {
		return ""
	}
	
	// Try common patterns for image specification
	patterns := [][]string{
		{"image", "repository"},
		{"image", "name"},
		{"image"},
		{"app", "image"},
		{"deployment", "image"},
	}
	
	for _, pattern := range patterns {
		if image := h.getNestedValue(values, pattern); image != "" {
			return image
		}
	}
	
	return ""
}

func (h *HelmSignal) containsImageConfig(values map[string]interface{}) bool {
	for key, value := range values {
		if key == "image" {
			return true
		}
		if subMap, ok := value.(map[string]interface{}); ok {
			if h.containsImageConfig(subMap) {
				return true
			}
		}
	}
	return false
}

func (h *HelmSignal) getNestedValue(values map[string]interface{}, keys []string) string {
	current := values
	
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - return the string value
			if str, ok := current[key].(string); ok {
				return str
			}
			return ""
		}
		
		// Navigate deeper
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	
	return ""
}

// parseChartYaml reads and parses a Chart.yaml file using the filesystem interface
func (h *HelmSignal) parseChartYaml(configPath string) (*chart.Metadata, error) {
	content, err := h.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var chart chart.Metadata
	if err := yaml.Unmarshal(content, &chart); err != nil {
		return nil, err
	}

	return &chart, nil
}

