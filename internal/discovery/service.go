package discovery

import (
	"context"
	"fmt"
	"slices"
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
	// Called for each file/directory entry encountered during directory walk
	ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error

	// Called after all entries in a directory have been observed to generate services
	GenerateServices(ctx context.Context) ([]types.Service, error)

	// Reset internal state before processing a new directory
	Reset()

	// Confidence level for conflict resolution
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
		signals.NewDigitalOceanAppSignal(filesystem),
		signals.NewVercelSignal(filesystem),
		signals.NewNetlifySignal(filesystem),
		signals.NewHerokuProcfileSignal(filesystem),
		signals.NewHerokuAppJsonSignal(filesystem),
		signals.NewHelmSignal(filesystem),
		signals.NewSkaffoldSignal(filesystem),
		signals.NewServerlessSignal(filesystem),
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

	// Reset all signals ONCE at the start
	for _, signal := range sd.signals {
		signal.Reset()
	}

	var lastCriticalError error

	// Walk the entire repo using a stack instead of recursion
	err := sd.walkRepoIterative(ctx, filesystem, basePath, 4, &lastCriticalError)
	if err != nil {
		return nil, fmt.Errorf("filesystem walk failed: %w", err)
	}

	// NOW generate services from all signals with their full accumulated context
	var results []signalResult
	for _, signal := range sd.signals {
		services, err := signal.GenerateServices(ctx)
		if err != nil {
			if isCriticalError(err) {
				lastCriticalError = err
			}
			continue
		}
		if len(services) > 0 {
			results = append(results, signalResult{
				services:   services,
				confidence: signal.Confidence(),
				signal:     signal,
			})
		}
	}

	// If we found no services but had critical errors, surface the error
	if len(results) == 0 && lastCriticalError != nil {
		return nil, fmt.Errorf("service discovery failed with authentication or permission error: %w", lastCriticalError)
	}

	// Merge services with confidence-based triangulation
	return triangulateServices(results), nil
}

func triangulateServices(results []signalResult) []types.Service {
	// Build service mapping by build path + name (keep distinct services)
	serviceMap := make(map[string][]serviceWithSignal)

	// Group services by their build path + name combination
	for _, result := range results {
		for _, service := range result.services {
			if service.BuildPath != "" {
				key := service.BuildPath + ":" + service.Name
				serviceMap[key] = append(serviceMap[key], serviceWithSignal{
					service:    service,
					confidence: result.confidence,
				})
			}
		}
	}

	var mergedServices []types.Service
	processed := make(map[string]bool)

	// Merge services with the same build path + name
	for key, serviceList := range serviceMap {
		if processed[key] {
			continue
		}

		// Use the highest confidence service as base, merge configs from all
		var bestService types.Service
		var allConfigs []types.ConfigRef
		configSet := make(map[string]bool) // for deduplication
		maxConfidence := 0

		for _, sws := range serviceList {
			for _, config := range sws.service.Configs {
				configKey := config.Type + ":" + config.Path
				if !configSet[configKey] {
					allConfigs = append(allConfigs, config)
					configSet[configKey] = true
				}
			}
			if sws.confidence > maxConfidence {
				maxConfidence = sws.confidence
				bestService = sws.service
			}
		}

		bestService.Configs = allConfigs
		mergedServices = append(mergedServices, bestService)
		processed[key] = true
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

var excludePatterns = []string{
	// Dependencies
	"node_modules", "vendor", "bower_components",
	"venv", "env",
	"target", "deps", "_build",

	// Build outputs
	"dist", "build", "out", ".next", ".nuxt", ".output",
	"public", "static", "assets", ".vercel", ".netlify",
	"bin", "obj", "Debug", "Release", "x64", "x86",

	// OS
	"Thumbs.db", "Desktop.ini",

	// Temporary
	"tmp", "temp", "cache", "logs", "coverage",

	// Usually not services
	"man", "examples", "test", "tests",
}

var includePatterns = []string{".do", ".vercel"}

func (sd *ServiceDiscovery) shouldIgnoreDirectory(dirName string) bool {
	// Check exact matches
	for _, pattern := range excludePatterns {
		if strings.EqualFold(dirName, pattern) {
			return true
		}
	}

	// Check prefixes
	if strings.HasPrefix(dirName, "_") || (strings.HasPrefix(dirName, ".") && len(dirName) > 1) && !slices.Contains(includePatterns, dirName) {
		// Allow "." (current dir) but ignore other dotfiles
		return true
	}

	return false
}

type walkItem struct {
	path  string
	depth int
}

// walkRepoIterative performs iterative directory traversal using a stack
func (sd *ServiceDiscovery) walkRepoIterative(ctx context.Context, filesystem fs.FileSystem, rootPath string, maxDepth int, lastCriticalError *error) error {
	// Use a stack instead of recursion
	stack := []walkItem{{path: rootPath, depth: 0}}

	for len(stack) > 0 {
		// Pop from stack
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if current.depth > maxDepth {
			continue
		}

		// Skip ignored directories
		dirName := filesystem.Base(current.path)
		if sd.shouldIgnoreDirectory(dirName) {
			continue
		}

		// Read directory and let signals observe ALL files
		for entry, err := range filesystem.ReadDir(current.path) {
			if err != nil {
				if isCriticalError(err) {
					*lastCriticalError = err
				}
				continue
			}

			// Let all signals observe this entry - they build up global repo state
			for _, signal := range sd.signals {
				if err := signal.ObserveEntry(ctx, current.path, entry); err != nil {
					if isCriticalError(err) {
						*lastCriticalError = err
					}
					// Continue with other signals even if one fails
				}
			}

			// Add subdirectories to stack for processing
			if entry.IsDir() {
				subPath := filesystem.Join(current.path, entry.Name())
				stack = append(stack, walkItem{path: subPath, depth: current.depth + 1})
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
