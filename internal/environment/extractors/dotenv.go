package extractors

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/railwayapp/turnout/internal/environment/types"
)

type DotEnvExtractor struct{}

func NewDotEnvExtractor() *DotEnvExtractor {
	return &DotEnvExtractor{}
}

func (d *DotEnvExtractor) CanHandle(filename string) bool {
	base := strings.ToLower(filepath.Base(filename))
	return strings.HasPrefix(base, ".env")
}

func (d *DotEnvExtractor) Confidence() int {
	return 85 // High confidence for explicit env files
}

func (d *DotEnvExtractor) Extract(ctx context.Context, filename string, content []byte) ([]types.EnvResult, error) {
	// Parse the dotenv content
	env, err := godotenv.Unmarshal(string(content))
	if err != nil {
		return nil, err
	}

	var results []types.EnvResult
	confidence := d.getFileConfidence(filepath.Base(filename))

	for key, value := range env {
		envType, sensitive := types.ClassifyEnvVar(key, value)
		results = append(results, types.EnvResult{
			VarName:    key,
			Value:      value,
			Type:       envType,
			Sensitive:  sensitive,
			Source:     fmt.Sprintf("dotenv:%s", filename),
			Confidence: confidence,
		})
	}

	return results, nil
}

func (d *DotEnvExtractor) getFileConfidence(filename string) int {
	switch {
	case filename == ".env":
		return 85 // High confidence for main env file
	case strings.Contains(filename, "production"):
		return 90 // Very high for production
	case strings.Contains(filename, "example"):
		return 30 // Low confidence for example files
	default:
		return 75 // Good confidence for other env files
	}
}
