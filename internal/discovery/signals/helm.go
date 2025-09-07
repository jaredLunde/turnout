package signals

import (
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
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
		serviceName := h.deriveServiceName(chart, buildPath)

		// Only include charts that have container images
		image := h.extractImageFromHelm(buildPath)
		if image == "" {
			continue // Skip charts without images - not deployable services
		}

		service := types.Service{
			Name:      serviceName,
			Network:   h.determineNetworkFromHelm(buildPath),
			Runtime:   types.RuntimeContinuous,
			Build:     types.BuildFromImage,
			BuildPath: buildPath,
			Image:     image,
			Configs: []types.ConfigRef{
				{Type: "helm", Path: configPath},
			},
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
	// Try to extract primary image from values.yaml
	images := h.getAllImagesFromValues(chartDir)
	if len(images) > 0 {
		return images[0] // Return primary image for now
	}
	return ""
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
	images := h.getAllImagesFromValues(chartDir)
	if len(images) > 0 {
		return images[0]
	}
	return ""
}

func (h *HelmSignal) getAllImagesFromValues(chartDir string) []string {
	valuesPath := h.filesystem.Join(chartDir, "values.yaml")
	content, err := h.filesystem.ReadFile(valuesPath)
	if err != nil {
		return nil
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(content, &values); err != nil {
		return nil
	}

	var images []string
	imageSet := make(map[string]bool) // for deduplication

	// Try repository + name combination first (Pattern 3: cfssl-issuer style)
	if imageMap, ok := values["image"].(map[string]interface{}); ok {
		repository := h.getStringValue(imageMap, "repository")
		name := h.getStringValue(imageMap, "name")
		if repository != "" && name != "" {
			// Combine repository and name
			var fullImage string
			if strings.HasSuffix(repository, "/") {
				fullImage = repository + name
			} else {
				fullImage = repository + "/" + name
			}
			if !imageSet[fullImage] {
				images = append(images, fullImage)
				imageSet[fullImage] = true
			}
		}
	}

	// Try common patterns for image specification
	patterns := [][]string{
		{"image", "repository"}, // Pattern 1: helm-state-metrics style
		{"app", "image"},        // Pattern 2: kartotherian style
		{"image"},               // Simple image field
		{"deployment", "image"},
		// Pattern 5: Multiple components (cert-manager style)
		{"webhook", "image", "repository"},
		{"cainjector", "image", "repository"},
		{"acmesolver", "image", "repository"},
		{"startupapicheck", "image", "repository"},
	}

	for _, pattern := range patterns {
		if image := h.getNestedValue(values, pattern); image != "" {
			if !imageSet[image] {
				images = append(images, image)
				imageSet[image] = true
			}
		}
	}

	// Also recursively search for any field named "image" or ending in "image"
	h.findImagesRecursive(values, "", &images, imageSet)

	return images
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

func (h *HelmSignal) getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func (h *HelmSignal) findImagesRecursive(values map[string]interface{}, path string, images *[]string, imageSet map[string]bool) {
	for key, value := range values {
		fullPath := key
		if path != "" {
			fullPath = path + "." + key
		}

		// Check if this is an image field
		if strings.Contains(strings.ToLower(key), "image") {
			if str, ok := value.(string); ok && str != "" {
				// Skip common non-image fields
				if !strings.Contains(strings.ToLower(key), "pullpolicy") &&
					!strings.Contains(strings.ToLower(key), "tag") &&
					!strings.Contains(strings.ToLower(key), "version") &&
					len(str) > 0 && !imageSet[str] {
					*images = append(*images, str)
					imageSet[str] = true
				}
			}
		}

		// Recurse into nested maps
		if nestedMap, ok := value.(map[string]interface{}); ok {
			h.findImagesRecursive(nestedMap, fullPath, images, imageSet)
		}
	}
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
