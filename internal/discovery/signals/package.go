package signals

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
)

type PackageSignal struct {
	filesystem      fs.FileSystem
	currentRootPath string
	packagePaths    map[string]string // filename -> full path
}

func NewPackageSignal(filesystem fs.FileSystem) *PackageSignal {
	return &PackageSignal{filesystem: filesystem}
}

func (p *PackageSignal) Confidence() int {
	return 50 // Low confidence - dependencies might be unused or transitive
}

func (p *PackageSignal) Reset() {
	p.packagePaths = make(map[string]string)
	p.currentRootPath = ""
}

func (p *PackageSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	p.currentRootPath = rootPath
	
	if !entry.IsDir() {
		// Check for all package manager files
		packageFiles := []string{
			"package.json", "requirements.txt", "pyproject.toml", "go.mod", 
			"Cargo.toml", "composer.json", "Gemfile", "pom.xml", 
			"build.gradle", "build.gradle.kts", "Package.swift", "mix.exs",
		}
		
		for _, filename := range packageFiles {
			if strings.EqualFold(entry.Name(), filename) {
				fullPath := p.filesystem.Join(rootPath, entry.Name())
				p.packagePaths[filename] = fullPath
				break
			}
		}
		
		// Also check for *.csproj files
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".csproj") {
			fullPath := p.filesystem.Join(rootPath, entry.Name())
			p.packagePaths[entry.Name()] = fullPath
		}
	}
	
	return nil
}

func (p *PackageSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(p.packagePaths) == 0 {
		return nil, nil
	}

	frameworks := p.detectFrameworksFromPackages()

	var services []types.Service
	for _, fw := range frameworks {
		service := types.Service{
			Name:      p.filesystem.Base(p.currentRootPath),
			Network:   fw.Network,
			Runtime:   fw.Runtime,
			Build:     fw.Build,
			BuildPath: p.currentRootPath,
			Configs: []types.ConfigRef{
				{Type: "package", Path: fw.ConfigPath},
			},
		}
		services = append(services, service)
	}

	return services, nil
}

type PackageFramework struct {
	Name       string
	ConfigPath string
	Network    types.Network
	Runtime    types.Runtime
	Build      types.Build
}

func (p *PackageSignal) detectFrameworksFromPackages() []PackageFramework {
	var frameworks []PackageFramework

	// Node.js package.json
	if packagePath, found := p.packagePaths["package.json"]; found {
		if fw := p.analyzePackageJson(packagePath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// Python requirements.txt / pyproject.toml
	if requirementsPath, found := p.packagePaths["requirements.txt"]; found {
		if fw := p.analyzeRequirements(requirementsPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	if pyprojectPath, found := p.packagePaths["pyproject.toml"]; found {
		if fw := p.analyzePyProject(pyprojectPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// Go go.mod
	if goModPath, found := p.packagePaths["go.mod"]; found {
		if fw := p.analyzeGoMod(goModPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// Rust Cargo.toml
	if cargoPath, found := p.packagePaths["Cargo.toml"]; found {
		if fw := p.analyzeCargo(cargoPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// PHP composer.json
	if composerPath, found := p.packagePaths["composer.json"]; found {
		if fw := p.analyzeComposer(composerPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// Ruby Gemfile
	if gemfilePath, found := p.packagePaths["Gemfile"]; found {
		if fw := p.analyzeGemfile(gemfilePath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// Java/Kotlin/Scala
	if pomPath, found := p.packagePaths["pom.xml"]; found {
		if fw := p.analyzePom(pomPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// Check for gradle files
	if gradlePath, found := p.packagePaths["build.gradle"]; found {
		if fw := p.analyzeGradle(gradlePath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	} else if gradlePath, found := p.packagePaths["build.gradle.kts"]; found {
		if fw := p.analyzeGradle(gradlePath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// C#/.NET - check for any .csproj file
	for filename, path := range p.packagePaths {
		if strings.HasSuffix(strings.ToLower(filename), ".csproj") {
			if fw := p.analyzeCsproj(path); fw != nil {
				frameworks = append(frameworks, *fw)
			}
			break // Only process one csproj file
		}
	}

	// Swift Package.swift
	if swiftPath, found := p.packagePaths["Package.swift"]; found {
		if fw := p.analyzeSwiftPackage(swiftPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	// Elixir mix.exs
	if mixPath, found := p.packagePaths["mix.exs"]; found {
		if fw := p.analyzeMix(mixPath); fw != nil {
			frameworks = append(frameworks, *fw)
		}
	}

	return frameworks
}

func (p *PackageSignal) analyzePackageJson(packagePath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(packagePath)
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	allDeps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		allDeps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		allDeps[k] = v
	}

	// Frontend meta-frameworks (highest priority)
	if _, found := allDeps["next"]; found {
		return &PackageFramework{Name: "Next.js", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["nuxt"]; found {
		return &PackageFramework{Name: "Nuxt.js", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@remix-run/react"]; found {
		return &PackageFramework{Name: "Remix", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@sveltejs/kit"]; found {
		return &PackageFramework{Name: "SvelteKit", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["astro"]; found {
		return &PackageFramework{Name: "Astro", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["solid-start"]; found {
		return &PackageFramework{Name: "SolidStart", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@builder.io/qwik"]; found {
		return &PackageFramework{Name: "Qwik", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Static site generators
	if _, found := allDeps["gatsby"]; found {
		return &PackageFramework{Name: "Gatsby", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@docusaurus/core"]; found {
		return &PackageFramework{Name: "Docusaurus", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["vuepress"]; found {
		return &PackageFramework{Name: "VuePress", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@gridsome/cli"]; found {
		return &PackageFramework{Name: "Gridsome", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Build tools that might indicate frontend apps
	if _, found := allDeps["vite"]; found {
		return &PackageFramework{Name: "Vite App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["webpack"]; found {
		return &PackageFramework{Name: "Webpack App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["parcel"]; found {
		return &PackageFramework{Name: "Parcel App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["rollup"]; found {
		return &PackageFramework{Name: "Rollup App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Backend Node.js frameworks
	if _, found := allDeps["express"]; found {
		return &PackageFramework{Name: "Express.js", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["fastify"]; found {
		return &PackageFramework{Name: "Fastify", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["koa"]; found {
		return &PackageFramework{Name: "Koa.js", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["hapi"]; found {
		return &PackageFramework{Name: "Hapi.js", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@nestjs/core"]; found {
		return &PackageFramework{Name: "NestJS", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["apollo-server"]; found {
		return &PackageFramework{Name: "Apollo GraphQL", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@apollo/server"]; found {
		return &PackageFramework{Name: "Apollo GraphQL Server", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["strapi"]; found {
		return &PackageFramework{Name: "Strapi CMS", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@keystone-6/core"]; found {
		return &PackageFramework{Name: "Keystone.js", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Frontend frameworks/libraries
	if _, found := allDeps["react"]; found {
		return &PackageFramework{Name: "React App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["vue"]; found {
		return &PackageFramework{Name: "Vue.js App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["svelte"]; found {
		return &PackageFramework{Name: "Svelte App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["solid-js"]; found {
		return &PackageFramework{Name: "Solid.js App", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@angular/core"]; found {
		return &PackageFramework{Name: "Angular", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Mobile frameworks
	if _, found := allDeps["react-native"]; found {
		return &PackageFramework{Name: "React Native", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["expo"]; found {
		return &PackageFramework{Name: "Expo", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@ionic/react"]; found {
		return &PackageFramework{Name: "Ionic React", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@ionic/angular"]; found {
		return &PackageFramework{Name: "Ionic Angular", ConfigPath: packagePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Desktop frameworks
	if _, found := allDeps["electron"]; found {
		return &PackageFramework{Name: "Electron", ConfigPath: packagePath, Network: types.NetworkPrivate, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["@tauri-apps/api"]; found {
		return &PackageFramework{Name: "Tauri", ConfigPath: packagePath, Network: types.NetworkPrivate, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	return nil
}

func (p *PackageSignal) analyzeRequirements(requirementsPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(requirementsPath)
	if err != nil {
		return nil
	}

	content := strings.ToLower(string(data))

	// Web frameworks
	if strings.Contains(content, "django") {
		return &PackageFramework{Name: "Django", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "flask") {
		return &PackageFramework{Name: "Flask", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "fastapi") {
		return &PackageFramework{Name: "FastAPI", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "tornado") {
		return &PackageFramework{Name: "Tornado", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "sanic") {
		return &PackageFramework{Name: "Sanic", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "starlette") {
		return &PackageFramework{Name: "Starlette", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "quart") {
		return &PackageFramework{Name: "Quart", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "pyramid") {
		return &PackageFramework{Name: "Pyramid", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "bottle") {
		return &PackageFramework{Name: "Bottle", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "cherrypy") {
		return &PackageFramework{Name: "CherryPy", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Data/ML frameworks (might be APIs)
	if strings.Contains(content, "streamlit") {
		return &PackageFramework{Name: "Streamlit", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "dash") {
		return &PackageFramework{Name: "Dash", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "gradio") {
		return &PackageFramework{Name: "Gradio", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic Python web-related packages
	if strings.Contains(content, "requests") || strings.Contains(content, "urllib3") || strings.Contains(content, "httpx") {
		return &PackageFramework{Name: "Python Web Service", ConfigPath: requirementsPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	return nil
}

func (p *PackageSignal) analyzePyProject(pyprojectPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(pyprojectPath)
	if err != nil {
		return nil
	}

	content := strings.ToLower(string(data))

	// Look for dependencies in pyproject.toml
	if strings.Contains(content, "django") {
		return &PackageFramework{Name: "Django", ConfigPath: pyprojectPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "fastapi") {
		return &PackageFramework{Name: "FastAPI", ConfigPath: pyprojectPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "flask") {
		return &PackageFramework{Name: "Flask", ConfigPath: pyprojectPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	return &PackageFramework{Name: "Python Project", ConfigPath: pyprojectPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeGoMod(goModPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(goModPath)
	if err != nil {
		return nil
	}

	content := string(data)

	// Web frameworks
	if strings.Contains(content, "github.com/gin-gonic/gin") {
		return &PackageFramework{Name: "Gin", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "github.com/gofiber/fiber") {
		return &PackageFramework{Name: "Fiber", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "github.com/gorilla/mux") {
		return &PackageFramework{Name: "Gorilla Mux", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "github.com/labstack/echo") {
		return &PackageFramework{Name: "Echo", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "github.com/revel/revel") {
		return &PackageFramework{Name: "Revel", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "github.com/beego/beego") {
		return &PackageFramework{Name: "Beego", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "github.com/kataras/iris") {
		return &PackageFramework{Name: "Iris", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "go.uber.org/fx") {
		return &PackageFramework{Name: "Go Service (Fx)", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic Go service
	return &PackageFramework{Name: "Go Service", ConfigPath: goModPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeCargo(cargoPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(cargoPath)
	if err != nil {
		return nil
	}

	content := string(data)

	// Web frameworks
	if strings.Contains(content, "actix-web") {
		return &PackageFramework{Name: "Actix Web", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "warp") {
		return &PackageFramework{Name: "Warp", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "rocket") {
		return &PackageFramework{Name: "Rocket", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "axum") {
		return &PackageFramework{Name: "Axum", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "tide") {
		return &PackageFramework{Name: "Tide", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "poem") {
		return &PackageFramework{Name: "Poem", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "salvo") {
		return &PackageFramework{Name: "Salvo", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Desktop frameworks
	if strings.Contains(content, "tauri") {
		return &PackageFramework{Name: "Tauri", ConfigPath: cargoPath, Network: types.NetworkPrivate, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "egui") {
		return &PackageFramework{Name: "egui Desktop", ConfigPath: cargoPath, Network: types.NetworkPrivate, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic Rust service
	return &PackageFramework{Name: "Rust Service", ConfigPath: cargoPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeComposer(composerPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(composerPath)
	if err != nil {
		return nil
	}

	var composer struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}

	if err := json.Unmarshal(data, &composer); err != nil {
		return nil
	}

	allDeps := make(map[string]string)
	for k, v := range composer.Require {
		allDeps[k] = v
	}
	for k, v := range composer.RequireDev {
		allDeps[k] = v
	}

	// PHP frameworks
	if _, found := allDeps["laravel/framework"]; found {
		return &PackageFramework{Name: "Laravel", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["symfony/framework-bundle"]; found {
		return &PackageFramework{Name: "Symfony", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["cakephp/cakephp"]; found {
		return &PackageFramework{Name: "CakePHP", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["codeigniter4/framework"]; found {
		return &PackageFramework{Name: "CodeIgniter", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["zendframework/zendframework"]; found {
		return &PackageFramework{Name: "Zend Framework", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["laminas/laminas-mvc"]; found {
		return &PackageFramework{Name: "Laminas", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["yiisoft/yii2"]; found {
		return &PackageFramework{Name: "Yii2", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if _, found := allDeps["phalcon/phalcon"]; found {
		return &PackageFramework{Name: "Phalcon", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic PHP service
	return &PackageFramework{Name: "PHP Service", ConfigPath: composerPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeGemfile(gemfilePath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(gemfilePath)
	if err != nil {
		return nil
	}

	content := string(data)

	// Ruby frameworks
	if strings.Contains(content, "rails") {
		return &PackageFramework{Name: "Ruby on Rails", ConfigPath: gemfilePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "sinatra") {
		return &PackageFramework{Name: "Sinatra", ConfigPath: gemfilePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "hanami") {
		return &PackageFramework{Name: "Hanami", ConfigPath: gemfilePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "roda") {
		return &PackageFramework{Name: "Roda", ConfigPath: gemfilePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "grape") {
		return &PackageFramework{Name: "Grape API", ConfigPath: gemfilePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic Ruby service
	return &PackageFramework{Name: "Ruby Service", ConfigPath: gemfilePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzePom(pomPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(pomPath)
	if err != nil {
		return nil
	}

	content := string(data)

	// Java frameworks
	if strings.Contains(content, "spring-boot") {
		return &PackageFramework{Name: "Spring Boot", ConfigPath: pomPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "spring-framework") {
		return &PackageFramework{Name: "Spring Framework", ConfigPath: pomPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "quarkus") {
		return &PackageFramework{Name: "Quarkus", ConfigPath: pomPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "micronaut") {
		return &PackageFramework{Name: "Micronaut", ConfigPath: pomPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "vertx") {
		return &PackageFramework{Name: "Vert.x", ConfigPath: pomPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "dropwizard") {
		return &PackageFramework{Name: "Dropwizard", ConfigPath: pomPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic Java service
	return &PackageFramework{Name: "Java Service", ConfigPath: pomPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeGradle(gradlePath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(gradlePath)
	if err != nil {
		return nil
	}

	content := string(data)

	// Java/Kotlin frameworks
	if strings.Contains(content, "spring-boot") {
		return &PackageFramework{Name: "Spring Boot", ConfigPath: gradlePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "quarkus") {
		return &PackageFramework{Name: "Quarkus", ConfigPath: gradlePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "micronaut") {
		return &PackageFramework{Name: "Micronaut", ConfigPath: gradlePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "ktor") {
		return &PackageFramework{Name: "Ktor", ConfigPath: gradlePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Android
	if strings.Contains(content, "com.android.application") {
		return &PackageFramework{Name: "Android App", ConfigPath: gradlePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic JVM service
	return &PackageFramework{Name: "JVM Service", ConfigPath: gradlePath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeCsproj(csprojPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(csprojPath)
	if err != nil {
		return nil
	}

	content := string(data)

	// .NET frameworks
	if strings.Contains(content, "Microsoft.AspNetCore") {
		return &PackageFramework{Name: "ASP.NET Core", ConfigPath: csprojPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "Blazor") {
		return &PackageFramework{Name: "Blazor", ConfigPath: csprojPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "Microsoft.NET.Sdk.Web") {
		return &PackageFramework{Name: ".NET Web App", ConfigPath: csprojPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic .NET service
	return &PackageFramework{Name: ".NET Service", ConfigPath: csprojPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeSwiftPackage(swiftPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(swiftPath)
	if err != nil {
		return nil
	}

	content := string(data)

	// Swift frameworks
	if strings.Contains(content, "Vapor") {
		return &PackageFramework{Name: "Vapor", ConfigPath: swiftPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "Perfect") {
		return &PackageFramework{Name: "Perfect", ConfigPath: swiftPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "Kitura") {
		return &PackageFramework{Name: "Kitura", ConfigPath: swiftPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic Swift service
	return &PackageFramework{Name: "Swift Service", ConfigPath: swiftPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}

func (p *PackageSignal) analyzeMix(mixPath string) *PackageFramework {
	data, err := p.filesystem.ReadFile(mixPath)
	if err != nil {
		return nil
	}

	content := string(data)

	// Elixir frameworks
	if strings.Contains(content, "phoenix") {
		return &PackageFramework{Name: "Phoenix", ConfigPath: mixPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}
	if strings.Contains(content, "plug") {
		return &PackageFramework{Name: "Plug", ConfigPath: mixPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
	}

	// Generic Elixir service
	return &PackageFramework{Name: "Elixir Service", ConfigPath: mixPath, Network: types.NetworkPublic, Runtime: types.RuntimeContinuous, Build: types.BuildFromSource}
}


