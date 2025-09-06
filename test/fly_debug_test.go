package test

import (
	"context"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery/signals"
)

func TestFlySignalDebug(t *testing.T) {
	signal := &signals.FlySignal{}
	
	repoDir, err := GetTestRepo("https://github.com/fauna-labs/express-ts-fly-io-starter.git")
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}

	services, err := signal.Discover(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Fly signal failed: %v", err)
	}

	t.Logf("Fly signal found %d services", len(services))
	for _, service := range services {
		t.Logf("  - %s: %+v", service.Name, service)
	}
}