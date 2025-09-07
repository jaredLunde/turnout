package turnout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/railwayapp/turnout/internal/discovery"
	"github.com/railwayapp/turnout/internal/filesystems"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover [source-path]",
	Short: "Discover services in a project without running the full conversion pipeline",
	Long: `Discover scans the source tree and identifies all deployable services
without performing normalization, validation, or export steps. This is useful
for understanding what services exist in a project before running the full
turnout conversion process.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sourcePath := "."
		if len(args) > 0 {
			sourcePath = args[0]

			// If user provided a file path, use the parent directory
			if stat, err := os.Stat(sourcePath); err == nil && !stat.IsDir() {
				sourcePath = filepath.Dir(sourcePath)
			}
		}

		fmt.Printf("Discovering services in: %s\n", sourcePath)

		if err := runServiceDiscovery(sourcePath); err != nil {
			fmt.Printf("Service discovery failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func runServiceDiscovery(sourcePath string) error {
	// Create filesystem from the sourcePath (supports file://, github://, git://)
	filesystem, err := filesystems.NewFileSystem(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to create filesystem: %w", err)
	}

	// Clean up git filesystem if needed
	if gitFS, ok := filesystem.(*filesystems.GitFS); ok {
		defer gitFS.Cleanup()
	}

	// Service discovery - find and triangulate services from multiple signals
	serviceDiscovery := discovery.NewServiceDiscovery(filesystem)
	services, err := serviceDiscovery.Discover(context.Background(), sourcePath)
	if err != nil {
		return fmt.Errorf("service discovery failed: %w", err)
	}

	fmt.Printf("Discovered %d services:\n", len(services))
	for _, service := range services {
		fmt.Printf("  - %s: Network=%s, Runtime=%s, Build=%s\n",
			service.Name,
			networkToString(service.Network),
			runtimeToString(service.Runtime),
			buildToString(service.Build))

		if service.BuildPath != "" {
			fmt.Printf("    BuildPath: %s\n", service.BuildPath)
		}
		if service.Image != "" {
			fmt.Printf("    Image: %s\n", service.Image)
		}

		fmt.Printf("    Config sources (%d):\n", len(service.Configs))
		for _, config := range service.Configs {
			fmt.Printf("      - %s: %s\n", config.Type, config.Path)
		}
		fmt.Println()
	}

	// Export to JSON
	output, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON export failed: %w", err)
	}

	fmt.Printf("\nJSON Export:\n%s\n", string(output))
	return nil
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}
