package schema

// Project represents a complete deployment specification
type Project struct {
	Name     string    `json:"name"`
	Services []Service `json:"services"`
}

// Service represents a deployable workload
type Service struct {
	Name         string            `json:"name"`
	Image        string            `json:"image,omitempty"`
	SourcePath   string            `json:"sourcePath,omitempty"`
	Environment  map[string]EnvVar `json:"environment,omitempty"`
	Ports        []Port            `json:"ports,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
}

// EnvVar represents an environment variable with metadata
type EnvVar struct {
	Value     string `json:"value"`
	Sensitive bool   `json:"sensitive"`
}

// Port represents a network port configuration
type Port struct {
	Number   int  `json:"number"`
	IsPublic bool `json:"isPublic"`
}

// Constructors

func NewProject(name string) *Project {
	return &Project{
		Name:     name,
		Services: make([]Service, 0),
	}
}

func (p *Project) AddService(service Service) {
	p.Services = append(p.Services, service)
}

func NewService(name string) Service {
	return Service{
		Name:         name,
		Environment:  make(map[string]EnvVar),
		Ports:        make([]Port, 0),
		Dependencies: make([]string, 0),
	}
}

func NewEnvVar(value string, sensitive bool) EnvVar {
	return EnvVar{
		Value:     value,
		Sensitive: sensitive,
	}
}

func NewPort(number int, isPublic bool) Port {
	return Port{
		Number:   number,
		IsPublic: isPublic,
	}
}