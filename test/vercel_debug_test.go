package test

import (
	"context"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery/signals"
)

func TestVercelSignalDebug(t *testing.T) {
	signal := &signals.VercelSignal{}
	
	// Use the official Vercel repo
	repoDir, err := GetTestRepo("https://github.com/vercel/vercel.git")
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}
	
	services, err := signal.Discover(context.Background(), repoDir)
	if err != nil {
		// Expected - this repo might not have vercel.json
		t.Logf("No vercel.json found (expected): %v", err)
		return
	}

	t.Logf("Vercel signal found %d services", len(services))
	for _, service := range services {
		t.Logf("  - %s: %+v", service.Name, service)
	}
}