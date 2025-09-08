package extractors

import (
	"context"

	"github.com/railwayapp/turnout/internal/environment/types"
)

// ContentExtractor processes file content and extracts environment variables
type ContentExtractor interface {
	// Extract environment variables from file content
	Extract(ctx context.Context, filename string, content []byte) ([]types.EnvResult, error)

	// CanHandle returns true if this extractor can process the given file
	CanHandle(filename string) bool

	// Confidence returns the confidence level for this extractor (0-100)
	Confidence() int
}
