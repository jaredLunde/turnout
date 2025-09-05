package parser

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/railwayapp/turnout/internal/discovery"
	"github.com/railwayapp/turnout/internal/schema"
)

type DockerComposeParser struct{}

func (p *DockerComposeParser) CanParse(configType string) bool {
	return configType == "docker-compose"
}

func (p *DockerComposeParser) Parse(config discovery.ConfigFile) (ProjectFragment, error) {
	// Use compose-go to parse the file
	ctx := context.Background()
	projectName := filepath.Base(filepath.Dir(config.Path))
	
	options, err := cli.NewProjectOptions(
		[]string{config.Path},
		cli.WithOsEnv,
		cli.WithName(projectName),
	)
	if err != nil {
		return ProjectFragment{}, fmt.Errorf("failed to create project options: %w", err)
	}
	
	project, err := options.LoadProject(ctx)
	if err != nil {
		return ProjectFragment{}, fmt.Errorf("failed to load compose project: %w", err)
	}
	
	// Convert to our AST
	services := make([]schema.Service, 0, len(project.Services))
	for _, composeService := range project.Services {
		service, err := p.convertService(composeService)
		if err != nil {
			return ProjectFragment{}, fmt.Errorf("failed to convert service %s: %w", composeService.Name, err)
		}
		services = append(services, service)
	}
	
	return ProjectFragment{
		Name:     project.Name,
		Services: services,
		Source:   config.Path,
	}, nil
}

func (p *DockerComposeParser) convertService(composeService types.ServiceConfig) (schema.Service, error) {
	// Convert environment variables
	env := make(map[string]schema.EnvVar)
	for key, value := range composeService.Environment {
		if value == nil {
			continue
		}
		env[key] = schema.NewEnvVar(*value, p.isSensitive(key, *value))
	}
	
	// Convert ports
	ports := make([]schema.Port, 0, len(composeService.Ports))
	for _, port := range composeService.Ports {
		if port.Published == "" {
			continue
		}
		portNum, err := strconv.Atoi(strings.Split(port.Published, ":")[0])
		if err != nil {
			continue // skip malformed ports
		}
		ports = append(ports, schema.NewPort(portNum, true)) // Docker Compose ports are typically public
	}
	
	// Determine if using image or build
	var image, sourcePath string
	if composeService.Image != "" {
		image = composeService.Image
	} else if composeService.Build != nil {
		sourcePath = composeService.Build.Context
		if sourcePath == "" {
			sourcePath = "." // default build context
		}
	}
	
	// Convert dependencies
	dependencies := make([]string, 0, len(composeService.DependsOn))
	for dep := range composeService.DependsOn {
		dependencies = append(dependencies, dep)
	}
	
	service := schema.NewService(composeService.Name)
	service.Image = image
	service.SourcePath = sourcePath
	service.Environment = env
	service.Ports = ports
	service.Dependencies = dependencies
	
	return service, nil
}

// Simple heuristic to detect sensitive environment variables
func (p *DockerComposeParser) isSensitive(key, value string) bool {
	key = strings.ToLower(key)
	sensitivePatterns := []string{"password", "secret", "key", "token", "auth"}
	
	for _, pattern := range sensitivePatterns {
		if strings.Contains(key, pattern) {
			return true
		}
	}
	
	// Also check if value looks like a secret (long, random-looking string)
	if len(value) > 20 && strings.Contains(value, "://") && strings.Contains(value, "@") {
		return true // database URLs with credentials
	}
	
	return false
}