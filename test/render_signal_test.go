package test

import (
	"context"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery/signals"
)

func TestRenderSignal_WithExample(t *testing.T) {
	// Use a Render example repo - this one has a render.yaml
	repoDir, err := GetTestRepo("https://github.com/render-examples/sinatra.git")
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}

	signal := &signals.RenderSignal{}
	services, err := signal.Discover(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Render signal failed: %v", err)
	}

	t.Logf("Render signal found %d services:", len(services))
	for _, service := range services {
		t.Logf("  - %s: Network=%v, Runtime=%v, Build=%v", 
			service.Name, service.Network, service.Runtime, service.Build)
		if service.Image != "" {
			t.Logf("    Image: %s", service.Image)
		}
	}
}