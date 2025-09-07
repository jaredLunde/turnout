package extractors

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/railwayapp/turnout/internal/environment/types"
)

type DockerfileExtractor struct{}

func NewDockerfileExtractor() *DockerfileExtractor {
	return &DockerfileExtractor{}
}

func (d *DockerfileExtractor) CanHandle(filename string) bool {
	name := strings.ToLower(filename)
	return strings.Contains(name, "dockerfile")
}

func (d *DockerfileExtractor) Confidence() int {
	return 60
}

func (d *DockerfileExtractor) Extract(ctx context.Context, filename string, content []byte) ([]types.EnvResult, error) {
	ast, err := parser.Parse(strings.NewReader(string(content)))
	if err != nil {
		return nil, err
	}

	var results []types.EnvResult

	// Walk the AST looking for ENV instructions
	for _, child := range ast.AST.Children {
		if strings.ToUpper(child.Value) == "ENV" {
			envVars := d.parseEnvNode(child, filename)
			results = append(results, envVars...)
		}
	}

	return results, nil
}

func (d *DockerfileExtractor) parseEnvNode(node *parser.Node, dockerfilePath string) []types.EnvResult {
	if node.Next == nil {
		return nil
	}

	var results []types.EnvResult
	var args []string
	for n := node.Next; n != nil; n = n.Next {
		args = append(args, n.Value)
	}

	if len(args) == 0 {
		return nil
	}

	// Handle ENV key=value format
	if strings.Contains(args[0], "=") {
		for _, arg := range args {
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) == 2 {
					varName := parts[0]
					value := parts[1]

					if types.ShouldIgnore(varName) {
						continue
					}
					
					envType, sensitive := types.ClassifyEnvVar(varName, value)
					results = append(results, types.EnvResult{
						VarName:    varName,
						Value:      value,
						Type:       envType,
						Sensitive:  sensitive,
						Source:     fmt.Sprintf("dockerfile:%s", dockerfilePath),
						Confidence: d.Confidence(),
					})
				}
			}
		}
	} else if len(args) >= 2 {
		// Handle ENV key value format
		varName := args[0]
		value := strings.Join(args[1:], " ")

		envType, sensitive := types.ClassifyEnvVar(varName, value)
		results = append(results, types.EnvResult{
			VarName:    varName,
			Value:      value,
			Type:       envType,
			Sensitive:  sensitive,
			Source:     fmt.Sprintf("dockerfile:%s", dockerfilePath),
			Confidence: d.Confidence(),
		})
	}

	return results
}
