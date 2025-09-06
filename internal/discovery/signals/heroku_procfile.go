package signals

import (
	"bufio"
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type HerokuProcfileSignal struct {
	filesystem fs.FileSystem
}

func NewHerokuProcfileSignal(filesystem fs.FileSystem) *HerokuProcfileSignal {
	return &HerokuProcfileSignal{filesystem: filesystem}
}

func (h *HerokuProcfileSignal) Confidence() int {
	return 85 // High confidence - Procfiles define explicit process types
}

func (h *HerokuProcfileSignal) Discover(ctx context.Context, rootPath string, dirEntries []fs.DirEntry) ([]types.Service, error) {
	// Look for Procfile
	configPath, err := fs.FindFileInEntries(h.filesystem, rootPath, "Procfile", dirEntries)
	if err != nil || configPath == "" {
		return nil, err
	}

	processes, err := h.parseProcfile(configPath)
	if err != nil {
		return nil, err
	}

	var services []types.Service
	for processType, command := range processes {
		service := types.Service{
			Name:      processType,
			Network:   determineNetworkFromProcfile(processType),
			Runtime:   determineRuntimeFromProcfile(processType, command),
			Build:     types.BuildFromSource, // Heroku builds from source
			BuildPath: rootPath,
			Configs: []types.ConfigRef{
				{Type: "procfile", Path: configPath},
			},
		}
		services = append(services, service)
	}

	return services, nil
}

func (h *HerokuProcfileSignal) parseProcfile(configPath string) (map[string]string, error) {
	content, err := h.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	processes := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		processType := strings.TrimSpace(parts[0])
		command := strings.TrimSpace(parts[1])
		processes[processType] = command
	}

	return processes, scanner.Err()
}

func determineNetworkFromProcfile(processType string) types.Network {
	// Web processes are typically public
	if processType == "web" {
		return types.NetworkPublic
	}
	
	// Workers, schedulers, etc. are typically private
	return types.NetworkPrivate
}

func determineRuntimeFromProcfile(processType, command string) types.Runtime {
	// Check for cron-like commands or scheduling indicators
	if strings.Contains(command, "cron") || 
	   strings.Contains(command, "schedule") ||
	   processType == "scheduler" ||
	   processType == "cron" {
		return types.RuntimeScheduled
	}
	
	// Default to continuous
	return types.RuntimeContinuous
}