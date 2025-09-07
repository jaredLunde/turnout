package environment

import (
	"context"

	"github.com/railwayapp/turnout/internal/environment/extractors"
	"github.com/railwayapp/turnout/internal/environment/types"
	"github.com/railwayapp/turnout/internal/filesystems"
)

type Extractor struct {
	filesystem filesystems.FileSystem
	extractors []extractors.ContentExtractor
}

func NewExtractor(filesystem filesystems.FileSystem) *Extractor {
	return &Extractor{
		filesystem: filesystem,
		extractors: []extractors.ContentExtractor{
			extractors.NewDockerComposeExtractor(),
			extractors.NewDockerfileExtractor(),
			extractors.NewDotEnvExtractor(),
		},
	}
}

// Extract environment variables from file content
func (e *Extractor) Extract(ctx context.Context, filename string, content []byte) <-chan types.EnvResult {
	results := make(chan types.EnvResult, 32)

	go func() {
		defer close(results)

		// Apply all extractors that can handle this file
		for _, extractor := range e.extractors {
			if extractor.CanHandle(filename) {
				envResults, err := extractor.Extract(ctx, filename, content)
				if err != nil {
					continue
				}

				for _, result := range envResults {
					results <- result
				}
			}
		}
	}()

	return results
}
