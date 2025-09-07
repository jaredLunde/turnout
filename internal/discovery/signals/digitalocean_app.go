package signals

import (
	"context"
	"strings"

	"github.com/railwayapp/turnout/internal/discovery/types"
	"github.com/railwayapp/turnout/internal/utils/fs"
	"gopkg.in/yaml.v3"
)

type DigitalOceanAppSignal struct {
	filesystem  fs.FileSystem
	configPaths []string          // all found DigitalOcean app spec files
	configDirs  map[string]string // config path -> directory path
}

func NewDigitalOceanAppSignal(filesystem fs.FileSystem) *DigitalOceanAppSignal {
	return &DigitalOceanAppSignal{filesystem: filesystem}
}

func (d *DigitalOceanAppSignal) Confidence() int {
	return 95 // Highest confidence - App Platform specs are explicit production deployment specs
}

func (d *DigitalOceanAppSignal) Reset() {
	d.configPaths = nil
	d.configDirs = make(map[string]string)
}

func (d *DigitalOceanAppSignal) ObserveEntry(ctx context.Context, rootPath string, entry fs.DirEntry) error {
	if entry.IsDir() && entry.Name() == ".do" {
		// Check for .do/app.yaml
		appPath := d.filesystem.Join(rootPath, ".do", "app.yaml")
		if _, err := d.filesystem.ReadFile(appPath); err == nil {
			d.configPaths = append(d.configPaths, appPath)
			// BuildPath should be the directory containing .do (repo root)
			d.configDirs[appPath] = rootPath
		}
	} else if !entry.IsDir() {
		// Check for app.yaml or digitalocean-app.yaml in root
		if strings.EqualFold(entry.Name(), "app.yaml") || strings.EqualFold(entry.Name(), "digitalocean-app.yaml") {
			configPath := d.filesystem.Join(rootPath, entry.Name())
			d.configPaths = append(d.configPaths, configPath)
			d.configDirs[configPath] = rootPath
		}
	}

	return nil
}

func (d *DigitalOceanAppSignal) GenerateServices(ctx context.Context) ([]types.Service, error) {
	if len(d.configPaths) == 0 {
		return nil, nil
	}

	var allServices []types.Service

	for _, configPath := range d.configPaths {
		config, err := d.parseAppSpec(configPath)
		if err != nil {
			continue // Skip broken configs
		}

		buildPath := d.configDirs[configPath]

		// If config is in .do subdirectory, build from repo root (parent of .do)
		if strings.HasSuffix(buildPath, "/.do") || buildPath == ".do" {
			buildPath = d.filesystem.Dir(buildPath)
			if buildPath == "" || buildPath == "." {
				buildPath = "."
			}
		}

		// Add HTTP services
		for _, appService := range config.Services {
			service := types.Service{
				Name:      appService.Name,
				Network:   types.NetworkPublic, // Services are publicly accessible
				Runtime:   types.RuntimeContinuous,
				Build:     determineBuildFromDOApp(appService),
				BuildPath: buildPath, // DigitalOcean builds from repo root by default
				Configs: []types.ConfigRef{
					{Type: "digitalocean-app", Path: configPath},
				},
			}

			// Set image for prebuilt Docker images
			if appService.Image != nil && appService.Image.Registry != "" {
				service.Image = appService.Image.Registry
			}

			allServices = append(allServices, service)
		}

		// Add static sites
		for _, site := range config.StaticSites {
			service := types.Service{
				Name:      site.Name,
				Network:   types.NetworkPublic, // Static sites are public
				Runtime:   types.RuntimeContinuous,
				Build:     types.BuildFromSource,
				BuildPath: buildPath, // DigitalOcean builds from repo root by default
				Configs: []types.ConfigRef{
					{Type: "digitalocean-app", Path: configPath},
				},
			}
			allServices = append(allServices, service)
		}

		// Add workers
		for _, worker := range config.Workers {
			service := types.Service{
				Name:      worker.Name,
				Network:   types.NetworkNone, // Workers are background processes
				Runtime:   types.RuntimeContinuous,
				Build:     determineBuildFromDOWorker(worker),
				BuildPath: buildPath, // DigitalOcean builds from repo root by default
				Configs: []types.ConfigRef{
					{Type: "digitalocean-app", Path: configPath},
				},
			}

			// Set image for prebuilt Docker images
			if worker.Image != nil && worker.Image.Registry != "" {
				service.Image = worker.Image.Registry
			}

			allServices = append(allServices, service)
		}

		// Add jobs
		for _, job := range config.Jobs {
			service := types.Service{
				Name:      job.Name,
				Network:   types.NetworkNone, // Jobs are background tasks
				Runtime:   types.RuntimeScheduled,
				Build:     determineBuildFromDOJob(job),
				BuildPath: buildPath, // DigitalOcean builds from repo root by default
				Configs: []types.ConfigRef{
					{Type: "digitalocean-app", Path: configPath},
				},
			}

			// Set image for prebuilt Docker images
			if job.Image != nil && job.Image.Registry != "" {
				service.Image = job.Image.Registry
			}

			allServices = append(allServices, service)
		}

		// Add databases as services
		for _, db := range config.Databases {
			service := types.Service{
				Name:    db.Name,
				Network: types.NetworkPrivate, // Databases are private
				Runtime: types.RuntimeContinuous,
				Build:   types.BuildFromImage,
				Image:   determineDBImageFromEngine(db.Engine),
				Configs: []types.ConfigRef{
					{Type: "digitalocean-app", Path: configPath},
				},
			}
			allServices = append(allServices, service)
		}
	}

	return allServices, nil
}

// DOAppSpec represents the DigitalOcean App Platform app spec structure
type DOAppSpec struct {
	Name        string         `yaml:"name"`
	Region      string         `yaml:"region,omitempty"`
	Services    []DOAppService `yaml:"services,omitempty"`
	StaticSites []DOStaticSite `yaml:"static_sites,omitempty"`
	Workers     []DOWorker     `yaml:"workers,omitempty"`
	Jobs        []DOJob        `yaml:"jobs,omitempty"`
	Functions   []DOFunction   `yaml:"functions,omitempty"`
	Databases   []DODatabase   `yaml:"databases,omitempty"`
}

type DOAppService struct {
	Name             string          `yaml:"name"`
	InstanceCount    int             `yaml:"instance_count,omitempty"`
	InstanceSizeSlug string          `yaml:"instance_size_slug,omitempty"`
	GitHub           *DOGitHubSource `yaml:"github,omitempty"`
	GitLab           *DOGitLabSource `yaml:"gitlab,omitempty"`
	Image            *DOImageSource  `yaml:"image,omitempty"`
	EnvironmentSlug  string          `yaml:"environment_slug,omitempty"`
	BuildCommand     string          `yaml:"build_command,omitempty"`
	RunCommand       string          `yaml:"run_command,omitempty"`
	HTTPPort         int             `yaml:"http_port,omitempty"`
	Routes           []DORoute       `yaml:"routes,omitempty"`
	HealthCheck      *DOHealthCheck  `yaml:"health_check,omitempty"`
	EnvVars          []DOEnvVar      `yaml:"envs,omitempty"`
}

type DOStaticSite struct {
	Name          string          `yaml:"name"`
	GitHub        *DOGitHubSource `yaml:"github,omitempty"`
	GitLab        *DOGitLabSource `yaml:"gitlab,omitempty"`
	BuildCommand  string          `yaml:"build_command,omitempty"`
	OutputDir     string          `yaml:"output_dir,omitempty"`
	IndexDocument string          `yaml:"index_document,omitempty"`
	ErrorDocument string          `yaml:"error_document,omitempty"`
	Routes        []DORoute       `yaml:"routes,omitempty"`
	EnvVars       []DOEnvVar      `yaml:"envs,omitempty"`
}

type DOWorker struct {
	Name             string          `yaml:"name"`
	InstanceCount    int             `yaml:"instance_count,omitempty"`
	InstanceSizeSlug string          `yaml:"instance_size_slug,omitempty"`
	GitHub           *DOGitHubSource `yaml:"github,omitempty"`
	GitLab           *DOGitLabSource `yaml:"gitlab,omitempty"`
	Image            *DOImageSource  `yaml:"image,omitempty"`
	EnvironmentSlug  string          `yaml:"environment_slug,omitempty"`
	BuildCommand     string          `yaml:"build_command,omitempty"`
	RunCommand       string          `yaml:"run_command,omitempty"`
	EnvVars          []DOEnvVar      `yaml:"envs,omitempty"`
}

type DOJob struct {
	Name            string          `yaml:"name"`
	Kind            string          `yaml:"kind,omitempty"` // PRE_DEPLOY, POST_DEPLOY, FAILED_DEPLOY
	GitHub          *DOGitHubSource `yaml:"github,omitempty"`
	GitLab          *DOGitLabSource `yaml:"gitlab,omitempty"`
	Image           *DOImageSource  `yaml:"image,omitempty"`
	EnvironmentSlug string          `yaml:"environment_slug,omitempty"`
	BuildCommand    string          `yaml:"build_command,omitempty"`
	RunCommand      string          `yaml:"run_command,omitempty"`
	EnvVars         []DOEnvVar      `yaml:"envs,omitempty"`
}

type DOFunction struct {
	Name            string          `yaml:"name"`
	GitHub          *DOGitHubSource `yaml:"github,omitempty"`
	GitLab          *DOGitLabSource `yaml:"gitlab,omitempty"`
	EnvironmentSlug string          `yaml:"environment_slug,omitempty"`
	Routes          []DORoute       `yaml:"routes,omitempty"`
	EnvVars         []DOEnvVar      `yaml:"envs,omitempty"`
}

type DODatabase struct {
	Name       string `yaml:"name"`
	Engine     string `yaml:"engine"` // PG, MYSQL, REDIS, etc.
	Version    string `yaml:"version,omitempty"`
	NumNodes   int    `yaml:"num_nodes,omitempty"`
	Size       string `yaml:"size,omitempty"`
	Production bool   `yaml:"production,omitempty"`
}

type DOGitHubSource struct {
	Repo         string `yaml:"repo"`
	Branch       string `yaml:"branch,omitempty"`
	DeployOnPush bool   `yaml:"deploy_on_push,omitempty"`
}

type DOGitLabSource struct {
	Repo         string `yaml:"repo"`
	Branch       string `yaml:"branch,omitempty"`
	DeployOnPush bool   `yaml:"deploy_on_push,omitempty"`
}

type DOImageSource struct {
	Registry     string `yaml:"registry"`
	Repository   string `yaml:"repository,omitempty"`
	Tag          string `yaml:"tag,omitempty"`
	RegistryType string `yaml:"registry_type,omitempty"`
}

type DORoute struct {
	Path               string `yaml:"path,omitempty"`
	PreservePathPrefix bool   `yaml:"preserve_path_prefix,omitempty"`
}

type DOHealthCheck struct {
	HTTPPath            string `yaml:"http_path,omitempty"`
	InitialDelaySeconds int    `yaml:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int    `yaml:"period_seconds,omitempty"`
	TimeoutSeconds      int    `yaml:"timeout_seconds,omitempty"`
	SuccessThreshold    int    `yaml:"success_threshold,omitempty"`
	FailureThreshold    int    `yaml:"failure_threshold,omitempty"`
}

type DOEnvVar struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value,omitempty"`
	Scope string `yaml:"scope,omitempty"` // RUN_TIME, BUILD_TIME, RUN_AND_BUILD_TIME
	Type  string `yaml:"type,omitempty"`  // GENERAL, SECRET
}

func (d *DigitalOceanAppSignal) parseAppSpec(configPath string) (*DOAppSpec, error) {
	data, err := d.filesystem.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config DOAppSpec
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func determineBuildFromDOApp(service DOAppService) types.Build {
	if service.Image != nil && service.Image.Registry != "" {
		return types.BuildFromImage
	}
	return types.BuildFromSource
}

func determineBuildFromDOWorker(worker DOWorker) types.Build {
	if worker.Image != nil && worker.Image.Registry != "" {
		return types.BuildFromImage
	}
	return types.BuildFromSource
}

func determineBuildFromDOJob(job DOJob) types.Build {
	if job.Image != nil && job.Image.Registry != "" {
		return types.BuildFromImage
	}
	return types.BuildFromSource
}

func determineDBImageFromEngine(engine string) string {
	switch strings.ToUpper(engine) {
	case "PG", "POSTGRES", "POSTGRESQL":
		return "postgres"
	case "MYSQL":
		return "mysql"
	case "REDIS":
		return "redis"
	case "MONGODB", "MONGO":
		return "mongo"
	default:
		return engine
	}
}
