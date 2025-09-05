package parser

import (
	"fmt"
	"path/filepath"

	"github.com/railwayapp/turnout/internal/schema"
)

type DefaultAggregator struct{}

func (a *DefaultAggregator) Aggregate(fragments []ProjectFragment) (*schema.Project, error) {
	if len(fragments) == 0 {
		return nil, fmt.Errorf("no project fragments to aggregate")
	}
	
	// Use the first fragment's name as the project name
	// Could be made smarter later (e.g., prefer certain sources)
	projectName := fragments[0].Name
	if projectName == "" {
		// Fallback to directory name from first fragment's source
		projectName = filepath.Base(filepath.Dir(fragments[0].Source))
	}
	
	project := schema.NewProject(projectName)
	
	// Merge all services from all fragments
	serviceNames := make(map[string]bool)
	for _, fragment := range fragments {
		for _, service := range fragment.Services {
			// Check for naming conflicts
			if serviceNames[service.Name] {
				return nil, fmt.Errorf("duplicate service name '%s' found in multiple config sources", service.Name)
			}
			serviceNames[service.Name] = true
			project.AddService(service)
		}
	}
	
	return project, nil
}

// NewAggregator creates a new default aggregator
func NewAggregator() Aggregator {
	return &DefaultAggregator{}
}