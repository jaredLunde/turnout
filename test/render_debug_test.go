package test

import (
	"context"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery/signals"
)

func TestRenderSignalDebug(t *testing.T) {
	signal := &signals.RenderSignal{}
	
	// Use the actual Render example repo
	repoDir, err := GetTestRepo("https://github.com/render-examples/patient-api.git")
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}
	
	services, err := signal.Discover(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Render signal failed: %v", err)
	}

	t.Logf("Render signal found %d services", len(services))
	for _, service := range services {
		t.Logf("  - %s: %+v", service.Name, service)
	}
}