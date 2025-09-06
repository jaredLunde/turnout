package discovery

import (
	"context"

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
	var results []signalResult

	for _, signal := range sd.signals {
		services, err := signal.Discover(ctx, rootPath)
		if err != nil {
			continue // Skip failed signals, don't fail entire discovery
		}
		results = append(results, signalResult{
			services:   services, 
			confidence: signal.Confidence(),
			signal:     signal,
		})
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
