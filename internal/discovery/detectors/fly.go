package detectors

import (
	"os"
	"strings"
)

type Fly struct{}

func (d *Fly) Name() string { return "fly" }

func (d *Fly) Detect(filename, fullPath string, info os.FileInfo) bool {
	return strings.ToLower(filename) == "fly.toml"
}
