package environment_test

import (
	"context"
	"testing"

	"github.com/railwayapp/turnout/internal/environment"
	"github.com/railwayapp/turnout/internal/environment/extractors"
	"github.com/railwayapp/turnout/internal/environment/types"
	"github.com/railwayapp/turnout/internal/filesystems"
)

func TestExtractor_DockerCompose(t *testing.T) {
	fs := filesystems.NewMemoryFS()
	extractor := environment.NewExtractor(fs)
	ctx := context.Background()

	composeContent := `services:
  web:
    image: nginx
    environment:
      PORT: 3000
      DATABASE_URL: postgres://user:pass@localhost/db
      JWT_SECRET: super-secret-key
      DEBUG: true
      API_KEY:
`

	// Test if extractor can handle the file
	dockerComposeExtractor := extractors.NewDockerComposeExtractor()
	canHandle := dockerComposeExtractor.CanHandle("docker-compose.yml")
	t.Logf("Can handle docker-compose.yml: %t", canHandle)
	
	// Test direct extraction
	directResults, err := dockerComposeExtractor.Extract(ctx, "docker-compose.yml", []byte(composeContent))
	if err != nil {
		t.Logf("Direct extraction error: %v", err)
	} else {
		t.Logf("Direct extraction found %d vars", len(directResults))
	}
	
	results := []types.EnvResult{}
	for result := range extractor.Extract(ctx, "docker-compose.yml", []byte(composeContent)) {
		t.Logf("Found env var: %s=%s", result.VarName, result.Value)
		results = append(results, result)
	}

	if len(results) != 5 {
		t.Fatalf("Expected 5 env vars, got %d", len(results))
	}

	// Verify some classifications
	foundDatabase := false
	foundSecret := false
	for _, result := range results {
		if result.VarName == "DATABASE_URL" && result.Sensitive {
			foundDatabase = true
		}
		if result.VarName == "JWT_SECRET" && result.Sensitive {
			foundSecret = true
		}
	}

	if !foundDatabase {
		t.Error("DATABASE_URL should be classified as sensitive")
	}
	if !foundSecret {
		t.Error("JWT_SECRET should be classified as sensitive")
	}
}

func TestExtractor_DotEnv(t *testing.T) {
	fs := filesystems.NewMemoryFS()
	extractor := environment.NewExtractor(fs)
	ctx := context.Background()

	envContent := `API_KEY=abc123
REDIS_URL=redis://localhost:6379
FEATURE_FLAG=true
PORT=3000
`

	results := []types.EnvResult{}
	for result := range extractor.Extract(ctx, ".env", []byte(envContent)) {
		results = append(results, result)
	}

	if len(results) != 4 {
		t.Fatalf("Expected 4 env vars, got %d", len(results))
	}

	// Check that API_KEY is marked sensitive
	foundApiKey := false
	for _, result := range results {
		if result.VarName == "API_KEY" && result.Sensitive {
			foundApiKey = true
			break
		}
	}

	if !foundApiKey {
		t.Error("API_KEY should be classified as sensitive")
	}
}