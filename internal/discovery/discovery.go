package discovery

import (
	"os"
	"path/filepath"
)

// ConfigFile represents a discovered deployment configuration file
type ConfigFile struct {
	Path string
	Type string // platform name like "docker-compose", "fly", "render"
}

// Detector defines the interface for platform-specific config detection
type Detector interface {
	Name() string // platform name
	Detect(filename, fullPath string, info os.FileInfo) bool
}

// Scanner handles recursive discovery using registered detectors
type Scanner struct {
	detectors []Detector
}

func NewScanner() *Scanner {
	return &Scanner{detectors: make([]Detector, 0)}
}

func (s *Scanner) RegisterDetector(detector Detector) {
	s.detectors = append(s.detectors, detector)
}

func (s *Scanner) DiscoverConfigs(rootPath string) ([]ConfigFile, error) {
	var configs []ConfigFile

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		for _, detector := range s.detectors {
			if detector.Detect(info.Name(), path, info) {
				configs = append(configs, ConfigFile{
					Path: path,
					Type: detector.Name(),
				})
				break // first match wins
			}
		}

		return nil
	})

	return configs, err
}

// NewScannerWithDetectors creates a scanner with the provided detectors
func NewScannerWithDetectors(detectors []Detector) *Scanner {
	scanner := NewScanner()
	for _, detector := range detectors {
		scanner.RegisterDetector(detector)
	}
	return scanner
}
