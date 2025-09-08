package extractors

import (
	"context"
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/railwayapp/turnout/internal/environment/types"
)

type DockerComposeExtractor struct{}

func NewDockerComposeExtractor() *DockerComposeExtractor {
	return &DockerComposeExtractor{}
}

func (d *DockerComposeExtractor) CanHandle(filename string) bool {
	name := strings.ToLower(filename)
	return strings.Contains(name, "compose") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"))
}

func (d *DockerComposeExtractor) Confidence() int {
	return 80
}

func (d *DockerComposeExtractor) Extract(ctx context.Context, filename string, content []byte) ([]types.EnvResult, error) {
	configDetails := composeTypes.ConfigDetails{
		WorkingDir: ".", // We don't need working dir for parsing
		ConfigFiles: []composeTypes.ConfigFile{
			{
				Filename: filename,
				Content:  content,
			},
		},
	}

	project, err := loader.LoadWithContext(ctx, configDetails, func(options *loader.Options) {
		options.SetProjectName("temp", true)
	})
	if err != nil {
		return nil, err
	}

	var results []types.EnvResult

	// Extract environment variables from all services
	for _, service := range project.Services {
		for key, value := range service.Environment {
			val := ""
			if value != nil {
				val = *value
			}

			if types.ShouldIgnore(key) {
				continue
			}

			envType, sensitive := types.ClassifyEnvVar(key, val)
			results = append(results, types.EnvResult{
				VarName:    key,
				Value:      val,
				Type:       envType,
				Sensitive:  sensitive,
				Source:     fmt.Sprintf("docker-compose:%s", filename),
				Confidence: d.Confidence(),
			})
		}
	}

	return results, nil
}
