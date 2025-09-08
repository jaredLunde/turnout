package extractors

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/railwayapp/turnout/internal/environment/types"
)

type LibraryCallExtractor struct{}

func NewLibraryCallExtractor() *LibraryCallExtractor {
	return &LibraryCallExtractor{}
}

// Common source code extensions
var sourceExts = []string{
	".js", ".ts", ".jsx", ".tsx", ".mjs",
	".py", ".rb", ".php", ".java", ".kt",
	".go", ".rs", ".cpp", ".c", ".cs",
	".sh", ".bash", ".zsh", ".fish",
}

func (l *LibraryCallExtractor) CanHandle(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	for _, sourceExt := range sourceExts {
		if ext == sourceExt {
			return true
		}
	}
	return false
}

func (l *LibraryCallExtractor) Confidence() int {
	return 50 // Medium confidence - these are usage patterns, not declarations
}

var libraryCallPatterns = []*regexp.Regexp{
	// process.env.VAR_NAME (JavaScript/TypeScript)
	regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),

	// os.getenv('VAR_NAME') or os.getenv("VAR_NAME") (Python)
	regexp.MustCompile(`os\.getenv\(['"]([A-Z_][A-Z0-9_]*)['"]\)`),

	// ENV['VAR_NAME'] or ENV["VAR_NAME"] (Ruby)
	regexp.MustCompile(`ENV\[['"]([A-Z_][A-Z0-9_]*)['"]\]`),

	// $_ENV['VAR_NAME'] or $_ENV["VAR_NAME"] (PHP)
	regexp.MustCompile(`\$_ENV\[['"]([A-Z_][A-Z0-9_]*)['"]\]`),

	// System.getenv("VAR_NAME") (Java)
	regexp.MustCompile(`System\.getenv\("([A-Z_][A-Z0-9_]*)"\)`),

	// os.Getenv("VAR_NAME") or os.LookupEnv("VAR_NAME") (Go)
	regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\("([A-Z_][A-Z0-9_]*)"\)`),

	// std::env::var("VAR_NAME") (Rust)
	regexp.MustCompile(`std::env::var\("([A-Z_][A-Z0-9_]*)"\)`),

	// $VAR_NAME (shell scripts) - but not in strings or comments
	regexp.MustCompile(`(?:^|[^#"'])\$([A-Z_][A-Z0-9_]*)`),

	// Environment.GetEnvironmentVariable("VAR_NAME") (C#)
	regexp.MustCompile(`Environment\.GetEnvironmentVariable\("([A-Z_][A-Z0-9_]*)"\)`),
}

func (l *LibraryCallExtractor) Extract(ctx context.Context, filename string, content []byte) ([]types.EnvResult, error) {
	contentStr := string(content)
	var results []types.EnvResult
	found := make(map[string]bool) // Deduplicate within this file

	// Skip files that look like tests
	if isTestFile(filename) {
		return results, nil
	}

	for _, pattern := range libraryCallPatterns {
		matches := pattern.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			varName := match[1]

			if found[varName] || types.ShouldIgnore(varName) {
				continue
			}

			found[varName] = true

			envType, sensitive := types.ClassifyEnvVar(varName, "")
			results = append(results, types.EnvResult{
				VarName:    varName,
				Value:      "", // We don't know the value from usage
				Type:       envType,
				Sensitive:  sensitive,
				Source:     fmt.Sprintf("usage:%s", filename),
				Confidence: l.Confidence(),
			})
		}
	}

	return results, nil
}

func isTestFile(filename string) bool {
	name := strings.ToLower(filename)
	return strings.Contains(name, "test") ||
		strings.Contains(name, "spec") ||
		strings.Contains(name, "_test") ||
		strings.HasSuffix(name, ".test.js") ||
		strings.HasSuffix(name, ".test.ts") ||
		strings.HasSuffix(name, ".spec.js") ||
		strings.HasSuffix(name, ".spec.ts")
}
