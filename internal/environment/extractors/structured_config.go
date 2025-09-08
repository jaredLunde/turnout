package extractors

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/railwayapp/turnout/internal/environment/types"
)

type StructuredConfigExtractor struct{}

func NewStructuredConfigExtractor() *StructuredConfigExtractor {
	return &StructuredConfigExtractor{}
}

func (s *StructuredConfigExtractor) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.ToLower(filepath.Base(filename))

	// Look for config-related files
	if strings.Contains(base, "config") || strings.Contains(base, "env") {
		return ext == ".go" || ext == ".ts" || ext == ".js" || ext == ".py"
	}

	// Or common extensions that might have config
	return ext == ".go" || ext == ".ts" || ext == ".js" || ext == ".py"
}

func (s *StructuredConfigExtractor) Confidence() int {
	return 85 // High confidence - these are intentional config declarations
}

var structuredPatterns = []*regexp.Regexp{
	// Go struct tags: `env:"VAR_NAME"`
	regexp.MustCompile(`env:"([A-Z_][A-Z0-9_]*)"`),

	// Go struct tags with defaults: `env:"VAR" envDefault:"value"`
	regexp.MustCompile(`env:"([A-Z_][A-Z0-9_]*)".*envDefault:"([^"]*)"`),

	// Zod schema: VAR_NAME: z.something()
	regexp.MustCompile(`([A-Z_][A-Z0-9_]*)\s*:\s*z`),

	// Joi object schema: Joi.object({ VAR_NAME: Joi.string() })
	regexp.MustCompile(`Joi\.object\([^}]*([A-Z_][A-Z0-9_]*)\s*:\s*Joi\.\w+`),

	// envalid.cleanEnv: VAR_NAME: str()
	regexp.MustCompile(`cleanEnv\([^}]*([A-Z_][A-Z0-9_]*)\s*:\s*\w+\(`),

	// ProcessEnv interface: interface ProcessEnv { VAR_NAME: string }
	regexp.MustCompile(`interface\s+ProcessEnv[^}]*([A-Z_][A-Z0-9_]*)\s*:\s*string`),

	// Pydantic Field: Field(env="VAR_NAME")
	regexp.MustCompile(`Field\(env="([A-Z_][A-Z0-9_]*)"`),

	// Spring @Value annotation: @Value("${VAR_NAME}")
	regexp.MustCompile(`@Value\("\$\{([A-Z_][A-Z0-9_]*)\}"\)`),
}

func (s *StructuredConfigExtractor) Extract(ctx context.Context, filename string, content []byte) ([]types.EnvResult, error) {
	contentStr := string(content)
	var results []types.EnvResult
	found := make(map[string]bool) // Deduplicate within this file

	for _, pattern := range structuredPatterns {
		matches := pattern.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			varName := match[1]
			defaultValue := ""
			if len(match) > 2 {
				defaultValue = match[2]
			}

			if found[varName] || types.ShouldIgnore(varName) {
				continue
			}

			found[varName] = true

			envType, sensitive := types.ClassifyEnvVar(varName, defaultValue)
			results = append(results, types.EnvResult{
				VarName:    varName,
				Value:      defaultValue, // Use default if we found one
				Type:       envType,
				Sensitive:  sensitive,
				Source:     fmt.Sprintf("config:%s", filename),
				Confidence: s.Confidence(),
			})
		}
	}

	return results, nil
}
