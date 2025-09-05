package turnout

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/railwayapp/turnout/internal/discovery"
	"github.com/railwayapp/turnout/internal/discovery/detectors"
	"github.com/railwayapp/turnout/internal/export"
	"github.com/railwayapp/turnout/internal/parser"
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
	// 1. Discovery - find all config files
	scanner := discovery.NewScannerWithDetectors([]discovery.Detector{
		&detectors.DockerCompose{},
		&detectors.Dockerfile{},
	})
	
	configs, err := scanner.DiscoverConfigs(sourcePath)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}
	
	fmt.Printf("Found %d config files:\n", len(configs))
	for _, config := range configs {
		fmt.Printf("  %s: %s\n", config.Type, config.Path)
	}
	
	// 2. Parse - convert each config to project fragments
	parsers := []parser.Parser{
		&parser.DockerComposeParser{},
	}
	
	var fragments []parser.ProjectFragment
	for _, config := range configs {
		for _, p := range parsers {
			if p.CanParse(config.Type) {
				fragment, err := p.Parse(config)
				if err != nil {
					return fmt.Errorf("failed to parse %s: %w", config.Path, err)
				}
				fragments = append(fragments, fragment)
				fmt.Printf("Parsed %s -> %d services\n", config.Path, len(fragment.Services))
				break
			}
		}
	}
	
	// 3. Aggregate - merge fragments into unified project
	aggregator := parser.NewAggregator()
	project, err := aggregator.Aggregate(fragments)
	if err != nil {
		return fmt.Errorf("aggregation failed: %w", err)
	}
	
	fmt.Printf("\nProject: %s\n", project.Name)
	fmt.Printf("Services: %d\n", len(project.Services))
	for _, service := range project.Services {
		fmt.Printf("  - %s", service.Name)
		if service.Image != "" {
			fmt.Printf(" (image: %s)", service.Image)
		}
		if service.SourcePath != "" {
			fmt.Printf(" (build: %s)", service.SourcePath)
		}
		fmt.Printf("\n")
	}
	
	// 4. Export - convert to target format
	exporter := export.NewJSONExporter()
	output, err := exporter.Export(project)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}
	
	fmt.Printf("\nExported to %s:\n", exporter.Name())
	fmt.Println(string(output))
	
	return nil
}
