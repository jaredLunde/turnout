package export

import (
	"encoding/json"

	"github.com/railwayapp/turnout/internal/schema"
)

type JSONExporter struct{}

func (e *JSONExporter) Name() string {
	return "json"
}

func (e *JSONExporter) Export(project *schema.Project) ([]byte, error) {
	return json.MarshalIndent(project, "", "  ")
}

func NewJSONExporter() Exporter {
	return &JSONExporter{}
}