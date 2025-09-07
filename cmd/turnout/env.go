package turnout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/railwayapp/turnout/internal/discovery"
	"github.com/railwayapp/turnout/internal/environment"
	"github.com/railwayapp/turnout/internal/environment/types"
	"github.com/railwayapp/turnout/internal/filesystems"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env [source-path]",
	Short: "Extract environment variables from discovered services",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sourcePath := "."
		if len(args) > 0 {
			sourcePath = args[0]

			if stat, err := os.Stat(sourcePath); err == nil && !stat.IsDir() {
				sourcePath = filepath.Dir(sourcePath)
			}
		}

		fmt.Printf("Extracting environment variables from: %s\n\n", sourcePath)

		if err := runEnvExtraction(sourcePath); err != nil {
			fmt.Printf("Environment extraction failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func runEnvExtraction(sourcePath string) error {
	filesystem, err := filesystems.NewFileSystem(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to create filesystem: %w", err)
	}

	if gitFS, ok := filesystem.(*filesystems.GitFS); ok {
		defer gitFS.Cleanup()
	}

	// First discover services
	serviceDiscovery := discovery.NewServiceDiscovery(filesystem)
	services, err := serviceDiscovery.Discover(context.Background(), sourcePath)
	if err != nil {
		return fmt.Errorf("service discovery failed: %w", err)
	}

	if len(services) == 0 {
		fmt.Println("No services found")
		return nil
	}

	// Collect all service BuildPaths to avoid crossing boundaries
	servicePaths := make(map[string]bool)
	for _, service := range services {
		if service.BuildPath != "" {
			servicePaths[service.BuildPath] = true
		}
	}

	// Create environment extractor
	envExtractor := environment.NewExtractor(filesystem)
	ctx := context.Background()

	for _, service := range services {
		fmt.Printf("=== %s ===\n", service.Name)
		
		envVars := make(map[string]types.EnvResult) // Deduplicate by variable name
		
		if service.BuildPath != "" {
			// Walk service directory recursively, avoiding other service paths
			err := filesystem.Walk(service.BuildPath, func(path string, info filesystems.FileInfo, err error) error {
				if err != nil {
					return nil // Skip files we can't access
				}

				// Skip if this is another service's directory
				if path != service.BuildPath && servicePaths[path] {
					return filesystems.SkipDir
				}

				if !info.IsDir() {
					// Read file and apply extractors
					content, err := filesystem.ReadFile(path)
					if err != nil {
						return nil
					}

					for envVar := range envExtractor.Extract(ctx, path, content) {
						// Deduplicate - keep highest confidence version
						existing, exists := envVars[envVar.VarName]
						if !exists || envVar.Confidence > existing.Confidence {
							envVars[envVar.VarName] = envVar
						}
					}
				}
				return nil
			})
			
			if err != nil {
				fmt.Printf("  Error walking directory: %v\n", err)
			}
		}

		if len(envVars) == 0 {
			fmt.Printf("  No environment variables found\n")
		} else {
			for _, envVar := range envVars {
				sensitiveMarker := ""
				if envVar.Sensitive {
					sensitiveMarker = " [SENSITIVE]"
				}
				fmt.Printf("  %s = %s\n", envVar.VarName, envVar.Value)
				fmt.Printf("    Source: %s%s\n", envVar.Source, sensitiveMarker)
			}
		}
		fmt.Println()
	}

	return nil
}

func envTypeToString(envType types.EnvType) string {
	switch envType {
	case types.EnvTypeSecret:
		return "secret"
	case types.EnvTypeDatabase:
		return "database"
	case types.EnvTypeGenerated:
		return "generated"
	case types.EnvTypeURL:
		return "url"
	case types.EnvTypeBoolean:
		return "boolean"
	case types.EnvTypeNumeric:
		return "numeric"
	case types.EnvTypeConfig:
		return "config"
	default:
		return "unknown"
	}
}

func init() {
	rootCmd.AddCommand(envCmd)
}