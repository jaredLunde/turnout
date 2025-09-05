package parser

import (
	"github.com/railwayapp/turnout/internal/discovery"
	"github.com/railwayapp/turnout/internal/schema"
)

// ProjectFragment represents partial project information from a single config source
type ProjectFragment struct {
	Name     string           // suggested project name (may be overridden by aggregator)
	Services []schema.Service // services defined by this config
	Source   string           // which config file this came from
}

// Parser defines the interface for parsing specific deployment configuration formats
type Parser interface {
	// Parse takes a single config file and returns a project fragment
	Parse(config discovery.ConfigFile) (ProjectFragment, error)
	
	// CanParse returns true if this parser can handle the given config type
	CanParse(configType string) bool
}

// Aggregator combines multiple project fragments into a unified project
type Aggregator interface {
	// Aggregate merges multiple fragments into a single project
	Aggregate(fragments []ProjectFragment) (*schema.Project, error)
}