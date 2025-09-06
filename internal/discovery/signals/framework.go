package signals

import (
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type FrameworkSignal struct {
	filesystem    fs.FileSystem
	observedFiles map[string]string // filename -> full path
	observedDirs  map[string]string // dirname -> full path
	configDirs    map[string]string // config path -> directory path
}

func NewFrameworkSignal(filesystem fs.FileSystem) *FrameworkSignal {
	return &FrameworkSignal{filesystem: filesystem}
}

func (f *FrameworkSignal) Confidence() int {
	return 85 // High confidence - explicit framework configs indicate deployment intent
}

func (f *FrameworkSignal) Reset() {
	f.observedFiles = make(map[string]string)
	f.observedDirs = make(map[string]string)
	f.configDirs = make(map[string]string)
}

func (f *FrameworkSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if entry.IsDir() {
		fullPath := f.filesystem.Join(rootPath, entry.Name())
		f.observedDirs[entry.Name()] = fullPath
	} else {
		fullPath := f.filesystem.Join(rootPath, entry.Name())
		f.observedFiles[entry.Name()] = fullPath
		f.configDirs[fullPath] = rootPath
	}

	return nil
}

func (f *FrameworkSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	frameworks := f.detectFrameworks()

	var services []types.Service
	for _, fw := range frameworks {
		buildPath := f.configDirs[fw.ConfigPath]
		service := types.Service{
			Name:      f.filesystem.Base(buildPath),
			Network:   fw.Network,
			Runtime:   fw.Runtime,
			Build:     fw.Build,
			BuildPath: buildPath,
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

func (f *FrameworkSignal) detectFrameworks() []Framework {
	var frameworks []Framework

	// Frontend frameworks (public web services)
	if configPath := f.findConfigFile("next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Next.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "nuxt.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Nuxt.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("vite.config.js", "vite.config.ts", "vite.config.mjs", "vite.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Vite", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("webpack.config.js", "webpack.config.ts", "webpack.config.mjs", "webpack.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Webpack", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("angular.json", ".angular-cli.json"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Angular", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("vue.config.js", "vue.config.ts", "vue.config.mjs", "vue.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Vue.js", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("svelte.config.js", "svelte.config.ts", "svelte.config.mjs", "svelte.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "SvelteKit", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("remix.config.js", "remix.config.ts", "remix.config.mjs", "remix.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Remix", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("astro.config.js", "astro.config.ts", "astro.config.mjs", "astro.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Astro", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("gatsby-config.js", "gatsby-config.ts", "gatsby-config.mjs", "gatsby-config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Gatsby", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	// Backend frameworks with explicit configs
	if configPath := f.findConfigFile("manage.py"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Django", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if f.hasDirectory("app") && f.hasFile("config.ru") {
		configPath := f.findConfigFile("config.ru")
		frameworks = append(frameworks, Framework{
			Name: "Rails", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("mix.exs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Phoenix", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	// Node.js frameworks
	if configPath := f.findConfigFile("nest-cli.json"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "NestJS", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	// Java frameworks
	if configPath := f.findConfigFile("pom.xml"); configPath != "" && f.hasSpringBootIndicators() {
		frameworks = append(frameworks, Framework{
			Name: "Spring Boot", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("build.gradle", "build.gradle.kts"); configPath != "" && f.hasSpringBootIndicators() {
		frameworks = append(frameworks, Framework{
			Name: "Spring Boot", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	// PHP frameworks
	if configPath := f.findConfigFile("artisan"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Laravel", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	// Static site generators
	// (Gatsby already handled above in frontend frameworks section)

	if configPath := f.findConfigFile("hugo.toml", "hugo.yaml", "config.toml", "config.yaml"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Hugo", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	if configPath := f.findConfigFile("tailwind.config.js", "tailwind.config.ts", "tailwind.config.mjs", "tailwind.config.cjs"); configPath != "" {
		frameworks = append(frameworks, Framework{
			Name: "Tailwind", ConfigPath: configPath,
			Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource,
		})
	}

	return frameworks
}

func (f *FrameworkSignal) findConfigFile(filenames ...string) string {
	for _, filename := range filenames {
		if fullPath, exists := f.observedFiles[filename]; exists {
			return fullPath
		}
		// Also check case-insensitive
		for observedFile, fullPath := range f.observedFiles {
			if strings.EqualFold(observedFile, filename) {
				return fullPath
			}
		}
	}
	return ""
}

func (f *FrameworkSignal) hasDirectory(dirName string) bool {
	if _, exists := f.observedDirs[dirName]; exists {
		return true
	}
	// Also check case-insensitive
	for observedDir := range f.observedDirs {
		if strings.EqualFold(observedDir, dirName) {
			return true
		}
	}
	return false
}

func (f *FrameworkSignal) hasFile(filename string) bool {
	return f.findConfigFile(filename) != ""
}

func (f *FrameworkSignal) hasSpringBootIndicators() bool {
	// Look for Spring Boot specific files/directories
	return f.hasDirectory("src/main/java") ||
		f.hasFile("application.properties") ||
		f.hasFile("application.yml") ||
		f.hasFile("application.yaml")
}
