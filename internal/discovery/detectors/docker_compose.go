package detectors

import (
	"os"
	"strings"
)

type DockerCompose struct{}

func (d *DockerCompose) Name() string { return "docker-compose" }

func (d *DockerCompose) Detect(filename, fullPath string, info os.FileInfo) bool {
	filename = strings.ToLower(filename)
	patterns := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, pattern := range patterns {
		if filename == pattern {
			return true
		}
	}
	return false
}
