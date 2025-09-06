package turnout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/railwayapp/turnout/internal/discovery"
	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "turnout [source-path]",
	Short: "Convert deployment configurations to Railway format",
	Long: `Turnout takes a source tree and runs the deployment conversion pipeline:
1. Parse - Find and parse deployment configs (Docker Compose, Kubernetes, etc.)
2. Normalize - Convert to unified intermediate representation
3. Validate/Enrich - Add semantic information and validate consistency
4. Export - Generate Railway deployment configuration`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sourcePath := "."
		if len(args) > 0 {
			sourcePath = args[0]
		}

		fmt.Printf("Processing source tree: %s\n", sourcePath)

		if err := runPipeline(sourcePath); err != nil {
			fmt.Printf("Pipeline failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.turnout.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".turnout")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func runPipeline(sourcePath string) error {
	// Service discovery - find and triangulate services from multiple signals
	serviceDiscovery := discovery.NewServiceDiscovery()
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

func networkToString(n types.Network) string {
	switch n {
	case types.NetworkNone:
		return "none"
	case types.NetworkPrivate:
		return "private"
	case types.NetworkPublic:
		return "public"
	default:
		return "unknown"
	}
}

func runtimeToString(r types.Runtime) string {
	switch r {
	case types.RuntimeContinuous:
		return "continuous"
	case types.RuntimeScheduled:
		return "scheduled"
	default:
		return "unknown"
	}
}

func buildToString(b types.Build) string {
	switch b {
	case types.BuildFromSource:
		return "source"
	case types.BuildFromImage:
		return "image"
	default:
		return "unknown"
	}
}
