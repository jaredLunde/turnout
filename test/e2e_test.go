package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2ESimpleWebApp(t *testing.T) {
	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "../bin/turnout", "../.")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build CLI: %v", err)
	}
	defer os.Remove("../bin/turnout")
	
	// Run CLI against fixture
	fixturePath := filepath.Join("..", "testdata", "fixtures", "simple-web-app")
	cmd := exec.Command("../bin/turnout", fixturePath)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI execution failed: %v\nOutput: %s", err, output)
	}
	
	outputStr := string(output)
	t.Logf("CLI Output:\n%s", outputStr)
	
	// Basic validation
	if !strings.Contains(outputStr, "Processing source tree:") {
		t.Errorf("Expected processing message in output")
	}
	
	if !strings.Contains(outputStr, fixturePath) {
		t.Errorf("Expected fixture path in output")
	}
}