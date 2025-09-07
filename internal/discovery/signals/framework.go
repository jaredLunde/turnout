package signals

import (
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type FrameworkSignal struct {
	filesystem fs.FileSystem
	frameworks []Framework       // detected frameworks
	configDirs map[string]string // config path -> directory path
}

func NewFrameworkSignal(filesystem fs.FileSystem) *FrameworkSignal {
	return &FrameworkSignal{filesystem: filesystem}
}

func (f *FrameworkSignal) Confidence() int {
	return 85 // High confidence - explicit framework configs indicate deployment intent
}

func (f *FrameworkSignal) Reset() {
	f.frameworks = nil
	f.configDirs = make(map[string]string)
}

func (f *FrameworkSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if entry.IsDir() {
		return nil
	}

	fullPath := f.filesystem.Join(rootPath, entry.Name())
	name := entry.Name()

	// Detect frameworks by config files
	var framework Framework
	switch {
	case matchesAny(name, "next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs"):
		framework = Framework{Name: "Next.js", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "nuxt.config.cjs"):
		framework = Framework{Name: "Nuxt.js", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "vite.config.js", "vite.config.ts", "vite.config.mjs", "vite.config.cjs"):
		framework = Framework{Name: "Vite", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "webpack.config.js", "webpack.config.ts", "webpack.config.mjs", "webpack.config.cjs"):
		framework = Framework{Name: "Webpack", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "angular.json", ".angular-cli.json"):
		framework = Framework{Name: "Angular", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "vue.config.js", "vue.config.ts", "vue.config.mjs", "vue.config.cjs"):
		framework = Framework{Name: "Vue.js", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "svelte.config.js", "svelte.config.ts", "svelte.config.mjs", "svelte.config.cjs"):
		framework = Framework{Name: "SvelteKit", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "remix.config.js", "remix.config.ts", "remix.config.mjs", "remix.config.cjs"):
		framework = Framework{Name: "Remix", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "astro.config.js", "astro.config.ts", "astro.config.mjs", "astro.config.cjs"):
		framework = Framework{Name: "Astro", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "gatsby-config.js", "gatsby-config.ts", "gatsby-config.mjs", "gatsby-config.cjs"):
		framework = Framework{Name: "Gatsby", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case name == "manage.py":
		framework = Framework{Name: "Django", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case name == "config.ru":
		framework = Framework{Name: "Rails", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case name == "mix.exs":
		framework = Framework{Name: "Phoenix", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case name == "nest-cli.json":
		framework = Framework{Name: "NestJS", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case name == "artisan":
		framework = Framework{Name: "Laravel", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, "hugo.toml", "hugo.yaml"):
		framework = Framework{Name: "Hugo", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case matchesAny(name, ".eleventy.js", "eleventy.config.js", ".eleventy.config.js"):
		framework = Framework{Name: "Eleventy", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	case name == "Caddyfile":
		framework = Framework{Name: "Caddy", ConfigPath: fullPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	default:
		return nil
	}

	f.frameworks = append(f.frameworks, framework)
	f.configDirs[fullPath] = rootPath
	return nil
}

func (f *FrameworkSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	var services []types.Service
	for _, fw := range f.frameworks {
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

func matchesAny(name string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.EqualFold(name, pattern) {
			return true
		}
	}
	return false
}
