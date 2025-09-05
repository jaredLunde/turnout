package detectors

import (
	"os"
	"strings"
)

type Dockerfile struct{}

func (d *Dockerfile) Name() string { return "dockerfile" }

func (d *Dockerfile) Detect(filename, fullPath string, info os.FileInfo) bool {
	filename = strings.ToLower(filename)
	return filename == "dockerfile" || strings.HasPrefix(filename, "dockerfile.")
}
