package discovery

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/signals"
	"github.com/railwayapp/turnout/internal/discovery/types"
)

type ServiceDiscovery struct {
	signals []ServiceSignal
}

type ServiceSignal interface {
	Discover(ctx context.Context, rootPath string) ([]types.Service, error)
	Confidence() int // 0-100, for conflict resolution
}

func NewServiceDiscovery(signals ...ServiceSignal) *ServiceDiscovery {
	if len(signals) == 0 {
		// Default signals
		signals = DefaultSignals()
	}
	
	return &ServiceDiscovery{
		signals: signals,
	}
}

func DefaultSignals() []ServiceSignal {
	return []ServiceSignal{
		&signals.DockerComposeSignal{},
		&signals.DockerfileSignal{},
		&signals.RailwaySignal{},
		&signals.FlySignal{},
		&signals.RenderSignal{},
		&signals.VercelSignal{},
		&signals.NetlifySignal{},
		&signals.HerokuProcfileSignal{},
		&signals.HerokuAppJsonSignal{},
		&signals.FrameworkSignal{},
		&signals.PackageSignal{},
	}
}

type signalResult struct {
	services   []types.Service
	confidence int
	signal     ServiceSignal
}

type serviceWithSignal struct {
	service    types.Service
	confidence int
}

func (sd *ServiceDiscovery) Discover(ctx context.Context, rootPath string) ([]types.Service, error) {
	// Find all potential service directories recursively
	serviceDirs := sd.findServiceDirectories(rootPath, 4)
	
	var results []signalResult
	
	// Run all signals on all discovered directories
	for _, dir := range serviceDirs {
		for _, signal := range sd.signals {
			services, err := signal.Discover(ctx, dir)
			if err != nil {
				continue // Skip failed signals, don't fail entire discovery
			}
			if len(services) > 0 {
				results = append(results, signalResult{
					services:   services, 
					confidence: signal.Confidence(),
					signal:     signal,
				})
			}
		}
	}

	// Merge services with confidence-based triangulation
	return triangulateServices(results), nil
}

func triangulateServices(results []signalResult) []types.Service {
	// Build service mapping by build context (most reliable indicator)
	buildPathMap := make(map[string][]serviceWithSignal)
	
	// Group services by their build path (if they have one)
	for _, result := range results {
		for _, service := range result.services {
			if service.BuildPath != "" {
				buildPathMap[service.BuildPath] = append(buildPathMap[service.BuildPath], serviceWithSignal{
					service:    service,
					confidence: result.confidence,
				})
			}
		}
	}
	
	var mergedServices []types.Service
	processed := make(map[string]bool)
	
	// Merge services with the same build path
	for buildPath, serviceList := range buildPathMap {
		if processed[buildPath] {
			continue
		}
		
		// Sort by confidence (highest first)
		// Use the highest confidence service as base, merge others
		var bestService types.Service
		var allConfigs []types.ConfigRef
		maxConfidence := 0
		
		for _, sws := range serviceList {
			allConfigs = append(allConfigs, sws.service.Configs...)
			if sws.confidence > maxConfidence {
				maxConfidence = sws.confidence
				bestService = sws.service
			}
		}
		
		bestService.Configs = allConfigs
		mergedServices = append(mergedServices, bestService)
		processed[buildPath] = true
	}
	
	// Add services without build paths (like pre-built images)
	for _, result := range results {
		for _, service := range result.services {
			if service.BuildPath == "" {
				mergedServices = append(mergedServices, service)
			}
		}
	}
	
	return mergedServices
}

func (sd *ServiceDiscovery) findServiceDirectories(rootPath string, maxDepth int) []string {
	var dirs []string
	
	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip directories we can't read
		}
		
		if !d.IsDir() {
			return nil
		}
		
		// Calculate depth relative to root
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}
		
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			return filepath.SkipDir
		}
		
		// Comprehensive ignore list
		dirName := d.Name()
		if sd.shouldIgnoreDirectory(dirName) {
			return filepath.SkipDir
		}
		
		// Add this directory as a potential service location
		dirs = append(dirs, path)
		return nil
	})
	
	if err != nil {
		// Fallback to just root if walking fails
		return []string{rootPath}
	}
	
	return dirs
}

func (sd *ServiceDiscovery) shouldIgnoreDirectory(dirName string) bool {
	// Comprehensive ignore list
	ignorePatterns := []string{
		// Version control
		".git", ".svn", ".hg", ".bzr",
		
		// Dependencies
		"node_modules", "vendor", "bower_components", 
		"__pycache__", ".pytest_cache", "venv", ".venv", "env", ".env",
		"target", "deps", "_build", ".mix",
		
		// Build outputs
		"dist", "build", "out", ".next", ".nuxt", ".output", 
		"public", "static", "assets", ".vercel", ".netlify",
		"bin", "obj", "Debug", "Release", "x64", "x86",
		
		// IDE/Editor
		".vscode", ".idea", ".vs", ".atom", ".sublime-project",
		".eclipse", ".metadata", "*.xcworkspace", "*.xcodeproj",
		
		// OS
		".DS_Store", "Thumbs.db", "Desktop.ini",
		
		// Temporary
		"tmp", "temp", ".tmp", ".temp", "cache", ".cache",
		"logs", ".logs", "coverage", ".coverage", ".nyc_output",
		
		// Config
		".sass-cache", ".parcel-cache", ".turborepo", 
		".rush", ".pnp", ".yarn",
		
		// Documentation (usually not services)
		"docs", "documentation", "doc", "man", "examples", "demo",
		"test", "tests", "__tests__", "spec", "__spec__",
	}
	
	// Check exact matches
	for _, pattern := range ignorePatterns {
		if strings.EqualFold(dirName, pattern) {
			return true
		}
	}
	
	// Check prefixes
	if strings.HasPrefix(dirName, ".") && len(dirName) > 1 {
		// Allow "." (current dir) but ignore other dotfiles
		return true
	}
	
	return false
}
