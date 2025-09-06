package test

import (
	"context"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery"
	"github.com/railwayapp/turnout/internal/discovery/signals"
)

func TestTriangulationDebug(t *testing.T) {
	repoDir, err := GetTestRepo("https://github.com/fauna-labs/express-ts-fly-io-starter.git")
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}

	// Test each signal individually
	flySignal := &signals.FlySignal{}
	flyServices, err := flySignal.Discover(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Fly signal failed: %v", err)
	}
	t.Logf("Fly signal found %d services:", len(flyServices))
	for _, svc := range flyServices {
		t.Logf("  - %s (build: %s)", svc.Name, svc.BuildPath)
	}

	dockerSignal := &signals.DockerfileSignal{}
	dockerServices, err := dockerSignal.Discover(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Dockerfile signal failed: %v", err)
	}
	t.Logf("Dockerfile signal found %d services:", len(dockerServices))
	for _, svc := range dockerServices {
		t.Logf("  - %s (build: %s)", svc.Name, svc.BuildPath)
	}

	// Test full triangulation
	serviceDiscovery := discovery.NewServiceDiscovery()
	services, err := serviceDiscovery.Discover(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Service discovery failed: %v", err)
	}

	t.Logf("Final triangulated services: %d", len(services))
	for _, svc := range services {
		t.Logf("  - %s: %d configs", svc.Name, len(svc.Configs))
		for _, cfg := range svc.Configs {
			t.Logf("    - %s", cfg.Type)
		}
	}
}