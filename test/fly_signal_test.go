package test

import (
	"context"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery"
)

func TestFlySignal_RealProject(t *testing.T) {
	// Get the Fly.io example repo
	repoDir, err := GetTestRepo("https://github.com/fauna-labs/express-ts-fly-io-starter.git")
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}

	// Test full service discovery
	serviceDiscovery := discovery.NewServiceDiscovery()
	services, err := serviceDiscovery.Discover(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Service discovery failed: %v", err)
	}

	t.Logf("Found %d services:", len(services))
	for _, service := range services {
		t.Logf("  - %s: Network=%v, Runtime=%v, Build=%v", 
			service.Name, service.Network, service.Runtime, service.Build)
		t.Logf("    Config sources: %d", len(service.Configs))
		for _, config := range service.Configs {
			t.Logf("      - %s: %s", config.Type, config.Path)
		}
	}
}