package signals

import (
	"context"
	"os"
	"path/filepath"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type FrameworkSignal struct{}

func (f *FrameworkSignal) Confidence() int {
	return 85 // High confidence - explicit framework configs indicate deployment intent
}

func (f *FrameworkSignal) Discover(ctx context.Context, rootPath string) ([]types.Service, error) {
	frameworks := f.detectFrameworks(rootPath)
	
	var services []types.Service
	for _, fw := range frameworks {
		service := types.Service{
			Name:      inferServiceNameFromPath(rootPath),
			Network:   fw.Network,
			Runtime:   fw.Runtime,
			Build:     fw.Build,
			BuildPath: rootPath,
			Configs: []types.ConfigRef{
				{Type: "framework", Path: fw.ConfigPath},
			},
		}
		services = append(services, service)
	}
	
	return services, nil
}

type Framework struct {
	Name       string
	ConfigPath string
	Network    types.Network
	Runtime    types.Runtime
	Build      types.Build
}

func (f *FrameworkSignal) detectFrameworks(rootPath string) []Framework {
	var frameworks []Framework
	
	// Frontend frameworks (public web services)
	if configPath := f.findConfigFile(rootPath, "next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Next.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "nuxt.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Nuxt.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "vite.config.js", "vite.config.ts", "vite.config.mjs", "vite.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Vite", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "webpack.config.js", "webpack.config.ts", "webpack.config.mjs", "webpack.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Webpack", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "angular.json", ".angular-cli.json"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Angular", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "vue.config.js", "vue.config.ts", "vue.config.mjs", "vue.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Vue.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "svelte.config.js", "svelte.config.ts", "svelte.config.mjs", "svelte.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "SvelteKit", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "remix.config.js", "remix.config.ts", "remix.config.mjs", "remix.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Remix", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "astro.config.js", "astro.config.ts", "astro.config.mjs", "astro.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Astro", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "gatsby-config.js", "gatsby-config.ts", "gatsby-config.mjs", "gatsby-config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Gatsby", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Backend frameworks with explicit configs
	if configPath := f.findConfigFile(rootPath, "manage.py"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Django", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if f.hasDirectory(rootPath, "app") && f.hasFile(rootPath, "config.ru") {
		configPath, _ := fs.FindFile(rootPath, "config.ru")
		frameworks = append(frameworks, Framework{
			Name: "Rails", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "mix.exs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Phoenix", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Node.js frameworks
	if configPath := f.findConfigFile(rootPath, "nest-cli.json"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "NestJS", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Java frameworks  
	if configPath := f.findConfigFile(rootPath, "pom.xml"); configPath != "" && f.hasSpringBootIndicators(rootPath) {
		frameworks = append(frameworks, Framework{
			Name: "Spring Boot", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "build.gradle", "build.gradle.kts"); configPath != "" && f.hasSpringBootIndicators(rootPath) {
		frameworks = append(frameworks, Framework{
			Name: "Spring Boot", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// PHP frameworks
	if configPath := f.findConfigFile(rootPath, "artisan"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Laravel", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Static site generators
	if configPath := f.findConfigFile(rootPath, "gatsby-config.js", "gatsby-config.ts"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Gatsby", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, "hugo.toml", "hugo.yaml", "config.toml", "config.yaml"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Hugo", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	return frameworks
}

func (f *FrameworkSignal) findConfigFile(rootPath string, filenames ...string) string {
	for _, filename := range filenames {
		if configPath, err := fs.FindFile(rootPath, filename); err == nil && configPath != "" {
			return configPath
		}
	}
	return ""
}

func (f *FrameworkSignal) hasDirectory(rootPath, dirName string) bool {
	dirPath := filepath.Join(rootPath, dirName)
	if stat, err := os.Stat(dirPath); err == nil && stat.IsDir() {
		return true
	}
	return false
}

func (f *FrameworkSignal) hasFile(rootPath, filename string) bool {
	return f.findConfigFile(rootPath, filename) != ""
}

func (f *FrameworkSignal) hasSpringBootIndicators(rootPath string) bool {
	// Look for Spring Boot specific files/directories
	return f.hasDirectory(rootPath, "src/main/java") || 
		   f.hasFile(rootPath, "application.properties") ||
		   f.hasFile(rootPath, "application.yml") ||
		   f.hasFile(rootPath, "application.yaml")
}