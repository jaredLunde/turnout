package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery"
)

func TestServiceTriangulation(t *testing.T) {
	// Get cached awesome-compose repo
	repoDir, err := GetTestRepo("https://github.com/docker/awesome-compose.git")
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}

	flaskProject := filepath.Join(repoDir, "flask")
	if _, err := os.Stat(flaskProject); os.IsNotExist(err) {
		t.Skip("Flask project not found")
	}

	// Test full service discovery with both signals
	serviceDiscovery := discovery.NewServiceDiscovery()
	services, err := serviceDiscovery.Discover(context.Background(), flaskProject)
	if err != nil {
		t.Fatalf("Service discovery failed: %v", err)
	}

	if len(services) != 1 {
		t.Logf("Found %d services:", len(services))
		for i, svc := range services {
			t.Logf("  Service %d: Name='%s', BuildPath='%s', Configs=%+v", i, svc.Name, svc.BuildPath, svc.Configs)
		}
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	webService := services[0]
	if webService.Name != "web" {
		t.Errorf("Expected service name 'web', got '%s'", webService.Name)
	}

	// Check that both signals contributed to this service
	t.Logf("Service configs: %+v", webService.Configs)
	
	hasComposeConfig := false
	hasDockerfileConfig := false
	
	for _, config := range webService.Configs {
		switch config.Type {
		case "docker-compose":
			hasComposeConfig = true
		case "dockerfile":
			hasDockerfileConfig = true
		}
	}

	if !hasComposeConfig {
		t.Error("Expected docker-compose config in merged service")
	}
	if !hasDockerfileConfig {
		t.Error("Expected dockerfile config in merged service")
	}

	// Verify BuildPath was properly set
	if webService.BuildPath == "" {
		t.Error("Expected BuildPath to be set")
	}

	t.Logf("Successfully merged service from %d config sources", len(webService.Configs))
}