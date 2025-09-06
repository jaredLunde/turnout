package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/signals"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type ServiceDiscovery struct {
	signals    []ServiceSignal
	filesystem fs.FileSystem
}

type ServiceSignal interface {
	Discover(ctx context.Context, rootPath string, dirEntries []fs.DirEntry) ([]types.Service, error)
	Confidence() int // 0-100, for conflict resolution
}

func NewServiceDiscovery(filesystem fs.FileSystem, signals ...ServiceSignal) *ServiceDiscovery {
	if len(signals) == 0 {
		signals = DefaultSignals(filesystem)
	}
	
	return &ServiceDiscovery{
		signals:    signals,
		filesystem: filesystem,
	}
}

func DefaultSignals(filesystem fs.FileSystem) []ServiceSignal {
	return []ServiceSignal{
		signals.NewDockerComposeSignal(filesystem),
		signals.NewDockerfileSignal(filesystem),
		signals.NewRailwaySignal(filesystem),
		signals.NewFlySignal(filesystem),
		signals.NewRenderSignal(filesystem),
		signals.NewVercelSignal(filesystem),
		signals.NewNetlifySignal(filesystem),
		signals.NewHerokuProcfileSignal(filesystem),
		signals.NewHerokuAppJsonSignal(filesystem),
		signals.NewFrameworkSignal(filesystem),
		signals.NewPackageSignal(filesystem),
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
	// Use the filesystem from the struct
	filesystem := sd.filesystem
	
	// Get the base path for the filesystem
	basePath := fs.GetBasePath(rootPath)
	
	var results []signalResult
	var lastCriticalError error
	
	// Do our own efficient walk that doesn't duplicate ReadDir calls
	err := sd.efficientWalk(filesystem, basePath, 0, 4, &results, &lastCriticalError, ctx)
	
	if err != nil {
		return nil, fmt.Errorf("filesystem walk failed: %w", err)
	}
	
	// If we found no services but had critical errors, surface the error
	if len(results) == 0 && lastCriticalError != nil {
		return nil, fmt.Errorf("service discovery failed with authentication or permission error: %w", lastCriticalError)
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

// efficientWalk performs recursive directory traversal with exactly one ReadDir per directory
func (sd *ServiceDiscovery) efficientWalk(filesystem fs.FileSystem, path string, depth, maxDepth int, results *[]signalResult, lastCriticalError *error, ctx context.Context) error {
	if depth > maxDepth {
		return nil
	}
	
	// Skip ignored directories
	dirName := filesystem.Base(path)
	if sd.shouldIgnoreDirectory(dirName) {
		return nil
	}
	
	// Read directory contents ONCE
	dirEntries, err := filesystem.ReadDir(path)
	if err != nil {
		if isCriticalError(err) {
			*lastCriticalError = err
		}
		return nil
	}
	
	// Run all signals on this directory with the same directory contents
	for _, signal := range sd.signals {
		services, err := signal.Discover(ctx, path, dirEntries)
		if err != nil {
			if isCriticalError(err) {
				*lastCriticalError = err
			}
			continue
		}
		if len(services) > 0 {
			*results = append(*results, signalResult{
				services:   services, 
				confidence: signal.Confidence(),
				signal:     signal,
			})
		}
	}
	
	// Recurse into subdirectories using the entries we already have
	for _, entry := range dirEntries {
		if entry.IsDir() {
			subPath := filesystem.Join(path, entry.Name())
			if err := sd.efficientWalk(filesystem, subPath, depth+1, maxDepth, results, lastCriticalError, ctx); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// isCriticalError determines if an error is critical (auth/permission) vs expected (not found)
func isCriticalError(err error) bool {
	if err == nil {
		return false
	}
	
	errMsg := strings.ToLower(err.Error())
	
	// GitHub API authentication errors
	if strings.Contains(errMsg, "401 unauthorized") ||
		strings.Contains(errMsg, "403 forbidden") ||
		strings.Contains(errMsg, "bad credentials") ||
		strings.Contains(errMsg, "token") ||
		strings.Contains(errMsg, "authentication") {
		return true
	}
	
	// Rate limiting
	if strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "api rate limit exceeded") {
		return true
	}
	
	// Network/connection issues
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network") {
		return true
	}
	
	return false
}

