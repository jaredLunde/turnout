package signals

import (
	"context"
	"iter"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type FrameworkSignal struct {
	filesystem fs.FileSystem
}

func NewFrameworkSignal(filesystem fs.FileSystem) *FrameworkSignal {
	return &FrameworkSignal{filesystem: filesystem}
}

func (f *FrameworkSignal) Confidence() int {
	return 85 // High confidence - explicit framework configs indicate deployment intent
}

func (f *FrameworkSignal) Discover(ctx context.Context, rootPath string, dirEntries iter.Seq2[fs.DirEntry, error]) ([]types.Service, error) {
	frameworks := f.detectFrameworks(rootPath, dirEntries)
	
	var services []types.Service
	for _, fw := range frameworks {
		service := types.Service{
			Name:      f.filesystem.Base(rootPath),
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

func (f *FrameworkSignal) detectFrameworks(rootPath string, dirEntries iter.Seq2[fs.DirEntry, error]) []Framework {
	var frameworks []Framework
	
	// Frontend frameworks (public web services)
	if configPath := f.findConfigFile(rootPath, dirEntries, "next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Next.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "nuxt.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Nuxt.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "vite.config.js", "vite.config.ts", "vite.config.mjs", "vite.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Vite", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "webpack.config.js", "webpack.config.ts", "webpack.config.mjs", "webpack.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Webpack", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "angular.json", ".angular-cli.json"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Angular", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "vue.config.js", "vue.config.ts", "vue.config.mjs", "vue.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Vue.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "svelte.config.js", "svelte.config.ts", "svelte.config.mjs", "svelte.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "SvelteKit", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "remix.config.js", "remix.config.ts", "remix.config.mjs", "remix.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Remix", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "astro.config.js", "astro.config.ts", "astro.config.mjs", "astro.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Astro", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "gatsby-config.js", "gatsby-config.ts", "gatsby-config.mjs", "gatsby-config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Gatsby", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Backend frameworks with explicit configs
	if configPath := f.findConfigFile(rootPath, dirEntries, "manage.py"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Django", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if f.hasDirectory(rootPath, "app", dirEntries) && f.hasFile(rootPath, "config.ru", dirEntries) {
		configPath, _ := fs.FindFileInEntries(f.filesystem, rootPath, "config.ru", dirEntries)
		frameworks = append(frameworks, Framework{
			Name: "Rails", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "mix.exs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Phoenix", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Node.js frameworks
	if configPath := f.findConfigFile(rootPath, dirEntries, "nest-cli.json"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "NestJS", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Java frameworks  
	if configPath := f.findConfigFile(rootPath, dirEntries, "pom.xml"); configPath != "" && f.hasSpringBootIndicators(rootPath, dirEntries) {
		frameworks = append(frameworks, Framework{
			Name: "Spring Boot", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "build.gradle", "build.gradle.kts"); configPath != "" && f.hasSpringBootIndicators(rootPath, dirEntries) {
		frameworks = append(frameworks, Framework{
			Name: "Spring Boot", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// PHP frameworks
	if configPath := f.findConfigFile(rootPath, dirEntries, "artisan"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Laravel", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	// Static site generators
	if configPath := f.findConfigFile(rootPath, dirEntries, "gatsby-config.js", "gatsby-config.ts"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Gatsby", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	if configPath := f.findConfigFile(rootPath, dirEntries, "hugo.toml", "hugo.yaml", "config.toml", "config.yaml"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Hugo", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}
	
	return frameworks
}

func (f *FrameworkSignal) findConfigFile(rootPath string, dirEntries iter.Seq2[fs.DirEntry, error], filenames ...string) string {
	for _, filename := range filenames {
		if configPath, err := fs.FindFileInEntries(f.filesystem, rootPath, filename, dirEntries); err == nil && configPath != "" {
			return configPath
		}
	}
	return ""
}

func (f *FrameworkSignal) hasDirectory(rootPath, dirName string, dirEntries iter.Seq2[fs.DirEntry, error]) bool {
	for entry, err := range dirEntries {
		if err != nil {
			continue
		}
		if entry.IsDir() && strings.EqualFold(entry.Name(), dirName) {
			return true
		}
	}
	return false
}

func (f *FrameworkSignal) hasFile(rootPath, filename string, dirEntries iter.Seq2[fs.DirEntry, error]) bool {
	return f.findConfigFile(rootPath, dirEntries, filename) != ""
}

func (f *FrameworkSignal) hasSpringBootIndicators(rootPath string, dirEntries iter.Seq2[fs.DirEntry, error]) bool {
	// Look for Spring Boot specific files/directories
	return f.hasDirectory(rootPath, "src/main/java", dirEntries) || 
		   f.hasFile(rootPath, "application.properties", dirEntries) ||
		   f.hasFile(rootPath, "application.yml", dirEntries) ||
		   f.hasFile(rootPath, "application.yaml", dirEntries)
}