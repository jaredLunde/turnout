package export

import "github.com/railwayapp/turnout/internal/schema"

// Exporter defines the interface for exporting projects to various formats
type Exporter interface {
	// Export converts a project to the target format
	Export(project *schema.Project) ([]byte, error)
	
	// Name returns the exporter name (e.g., "railway", "json", "kubernetes")
	Name() string
}