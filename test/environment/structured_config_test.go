package environment_test

import (
	"context"
	"os"
	"testing"

	"github.com/railwayapp/turnout/internal/environment/extractors"
	"github.com/railwayapp/turnout/internal/environment/types"
)

func TestDockerComposeExtractor(t *testing.T) {
	extractor := extractors.NewDockerComposeExtractor()
	ctx := context.Background()

	content, err := os.ReadFile("testdata/docker-compose.yml")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	results, err := extractor.Extract(ctx, "docker-compose.yml", content)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	expectedVars := []string{"PORT", "DATABASE_URL", "JWT_SECRET", "DEBUG", "API_KEY"}

	if len(results) != len(expectedVars) {
		t.Fatalf("Expected %d vars, got %d", len(expectedVars), len(results))
	}

	found := make(map[string]bool)
	for _, result := range results {
		found[result.VarName] = true
		if result.Confidence != 80 {
			t.Errorf("Expected confidence 80, got %d for %s", result.Confidence, result.VarName)
		}
	}

	for _, expected := range expectedVars {
		if !found[expected] {
			t.Errorf("Missing expected var: %s", expected)
		}
	}
}

func TestDockerfileExtractor(t *testing.T) {
	extractor := extractors.NewDockerfileExtractor()
	ctx := context.Background()

	content, err := os.ReadFile("testdata/Dockerfile")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	results, err := extractor.Extract(ctx, "Dockerfile", content)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	expectedVars := []string{"PORT", "DATABASE_URL", "JWT_SECRET", "DEBUG"}

	if len(results) != len(expectedVars) {
		t.Fatalf("Expected %d vars, got %d", len(expectedVars), len(results))
	}

	for _, result := range results {
		if result.Confidence != 60 {
			t.Errorf("Expected confidence 60, got %d for %s", result.Confidence, result.VarName)
		}
	}
}

func TestDotEnvExtractor(t *testing.T) {
	extractor := extractors.NewDotEnvExtractor()
	ctx := context.Background()

	content, err := os.ReadFile("testdata/.env")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	results, err := extractor.Extract(ctx, ".env", content)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	expectedVars := []string{"API_KEY", "REDIS_URL", "FEATURE_FLAG", "PORT"}

	if len(results) != len(expectedVars) {
		t.Fatalf("Expected %d vars, got %d", len(expectedVars), len(results))
	}

	for _, result := range results {
		if result.Confidence != 85 {
			t.Errorf("Expected confidence 85, got %d for %s", result.Confidence, result.VarName)
		}
	}
}

func TestLibraryCallExtractor(t *testing.T) {
	extractor := extractors.NewLibraryCallExtractor()
	ctx := context.Background()

	content, err := os.ReadFile("testdata/source_code.js")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	results, err := extractor.Extract(ctx, "app.js", content)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	expectedVars := []string{"PORT", "DATABASE_URL", "API_KEY", "NODE_ENV"}

	if len(results) != len(expectedVars) {
		t.Fatalf("Expected %d vars, got %d", len(expectedVars), len(results))
	}

	for _, result := range results {
		if result.Confidence != 50 {
			t.Errorf("Expected confidence 50, got %d for %s", result.Confidence, result.VarName)
		}
		if result.Value != "" {
			t.Errorf("Expected empty value for usage extraction, got %s for %s", result.Value, result.VarName)
		}
	}
}

func TestStructuredConfig_ZodSchema(t *testing.T) {
	extractor := extractors.NewStructuredConfigExtractor()
	ctx := context.Background()

	content, err := os.ReadFile("testdata/zod_schema.ts")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	results, err := extractor.Extract(ctx, "env.ts", content)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	expectedVars := []string{"NODE_ENV", "HOSTNAME", "PORT", "RAILWAY_PUBLIC_DOMAIN", "DASHBOARD_URL"}

	t.Logf("Found %d variables:", len(results))
	for _, result := range results {
		t.Logf("  %s", result.VarName)
	}

	if len(results) != len(expectedVars) {
		t.Fatalf("Expected %d vars, got %d", len(expectedVars), len(results))
	}

	found := make(map[string]bool)
	for _, result := range results {
		found[result.VarName] = true
		t.Logf("Found: %s", result.VarName)
	}

	for _, expected := range expectedVars {
		if !found[expected] {
			t.Errorf("Missing expected var: %s", expected)
		}
	}
}

func TestDeduplication(t *testing.T) {
	ctx := context.Background()

	// Read test data that has overlapping variables
	envContent, err := os.ReadFile("testdata/duplicate_vars.env")
	if err != nil {
		t.Fatalf("Failed to read env fixture: %v", err)
	}

	composeContent, err := os.ReadFile("testdata/duplicate_vars.yml")
	if err != nil {
		t.Fatalf("Failed to read compose fixture: %v", err)
	}

	// Create extractors
	envExtractor := extractors.NewDotEnvExtractor()
	composeExtractor := extractors.NewDockerComposeExtractor()

	// Extract from both sources
	envResults, err := envExtractor.Extract(ctx, ".env", envContent)
	if err != nil {
		t.Fatalf("Env extract failed: %v", err)
	}

	composeResults, err := composeExtractor.Extract(ctx, "docker-compose.yml", composeContent)
	if err != nil {
		t.Fatalf("Compose extract failed: %v", err)
	}

	// Collect all results and deduplicate based on confidence
	allResults := append(envResults, composeResults...)
	deduplicated := make(map[string]types.EnvResult)

	for _, result := range allResults {
		existing, exists := deduplicated[result.VarName]
		if !exists || result.Confidence > existing.Confidence {
			deduplicated[result.VarName] = result
		}
	}

	// Verify deduplication behavior
	if len(deduplicated) != 3 {
		t.Fatalf("Expected 3 unique variables, got %d", len(deduplicated))
	}

	// PORT should come from .env (confidence 85) over docker-compose (confidence 80)
	portResult, exists := deduplicated["PORT"]
	if !exists {
		t.Fatal("PORT variable missing from results")
	}
	if portResult.Value != "8080" {
		t.Errorf("Expected PORT=8080 from .env file, got %s", portResult.Value)
	}
	if portResult.Confidence != 85 {
		t.Errorf("Expected confidence 85 for PORT, got %d", portResult.Confidence)
	}

	// API_KEY should come from .env (confidence 85) over docker-compose (confidence 80)
	apiResult, exists := deduplicated["API_KEY"]
	if !exists {
		t.Fatal("API_KEY variable missing from results")
	}
	if apiResult.Value != "env-file-key" {
		t.Errorf("Expected API_KEY=env-file-key from .env file, got %s", apiResult.Value)
	}
	if apiResult.Confidence != 85 {
		t.Errorf("Expected confidence 85 for API_KEY, got %d", apiResult.Confidence)
	}

	// DATABASE_URL should only exist in docker-compose
	dbResult, exists := deduplicated["DATABASE_URL"]
	if !exists {
		t.Fatal("DATABASE_URL variable missing from results")
	}
	if dbResult.Value != "postgres://localhost/db" {
		t.Errorf("Expected DATABASE_URL from compose file, got %s", dbResult.Value)
	}
	if dbResult.Confidence != 80 {
		t.Errorf("Expected confidence 80 for DATABASE_URL, got %d", dbResult.Confidence)
	}
}
