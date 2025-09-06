package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/railwayapp/turnout/internal/discovery/signals"
)

const awesomeComposeRepo = "https://github.com/docker/awesome-compose.git"

func TestDockerComposeSignal_RealProjects(t *testing.T) {
	// Get cached repo (singleflight ensures only one clone)
	repoDir, err := GetTestRepo(awesomeComposeRepo)
	if err != nil {
		t.Fatalf("Failed to get test repo: %v", err)
	}

	signal := &signals.DockerComposeSignal{}

	// Test specific projects that should have docker-compose.yml
	testProjects := []struct {
		name     string
		path     string
		expected int // expected minimum services
	}{
		{"flask", "flask", 1},                     // simple flask app
		{"flask-redis", "flask-redis", 2},         // flask + redis
		{"django", "django", 1},                   // django only
		{"elasticsearch-logstash-kibana", "elasticsearch-logstash-kibana", 3}, // ELK stack
	}

	for _, project := range testProjects {
		t.Run(project.name, func(t *testing.T) {
			projectPath := filepath.Join(repoDir, project.path)

			// Check if project exists
			if _, err := os.Stat(projectPath); os.IsNotExist(err) {
				t.Skipf("Project %s not found, skipping", project.name)
				return
			}

			services, err := signal.Discover(context.Background(), projectPath)
			if err != nil {
				t.Fatalf("Failed to discover services in %s: %v", project.name, err)
			}

			if len(services) < project.expected {
				t.Errorf("Expected at least %d services in %s, got %d", project.expected, project.name, len(services))
			}

			t.Logf("Project %s discovered %d services:", project.name, len(services))
			for _, svc := range services {
				t.Logf("  - %s: Network=%v, Runtime=%v, Build=%v", 
					svc.Name, svc.Network, svc.Runtime, svc.Build)
				if svc.BuildPath != "" {
					t.Logf("    BuildPath: %s", svc.BuildPath)
				}
				if svc.Image != "" {
					t.Logf("    Image: %s", svc.Image)
				}
			}

			// Verify services have reasonable characteristics
			for _, svc := range services {
				if svc.Name == "" {
					t.Error("Service has empty name")
				}
			}
		})
	}
}