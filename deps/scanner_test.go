package deps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFromBuildInfo(t *testing.T) {
	result, err := ScanFromBuildInfo()
	if err != nil {
		t.Skipf("build info not available: %v", err)
	}

	if result.GoVersion == "" {
		t.Error("GoVersion empty")
	}
}

func TestScanGoMod(t *testing.T) {
	// Create a temporary go.mod
	dir := t.TempDir()
	modFile := filepath.Join(dir, "go.mod")
	content := `module example.com/test

go 1.21

require (
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.4
	golang.org/x/sys v0.15.0 // indirect
)
`
	if err := os.WriteFile(modFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanGoMod(modFile)
	if err != nil {
		t.Fatalf("ScanGoMod failed: %v", err)
	}

	if result.ModulePath != "example.com/test" {
		t.Errorf("ModulePath = %q", result.ModulePath)
	}
	if result.GoVersion != "1.21" {
		t.Errorf("GoVersion = %q", result.GoVersion)
	}
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if result.Direct != 2 {
		t.Errorf("Direct = %d, want 2", result.Direct)
	}
	if result.Indirect != 1 {
		t.Errorf("Indirect = %d, want 1", result.Indirect)
	}

	// check specific module
	found := false
	for _, m := range result.Modules {
		if m.Path == "golang.org/x/sys" {
			found = true
			if !m.Indirect {
				t.Error("golang.org/x/sys should be indirect")
			}
			if m.Version != "v0.15.0" {
				t.Errorf("version = %q", m.Version)
			}
		}
	}
	if !found {
		t.Error("golang.org/x/sys not found")
	}
}

func TestScanGoModNotFound(t *testing.T) {
	_, err := ScanGoMod("/nonexistent/go.mod")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFormatScanResult(t *testing.T) {
	result := ScanResult{
		ModulePath: "example.com/test",
		GoVersion:  "1.21",
		Modules: []Module{
			{Path: "github.com/pkg/errors", Version: "v0.9.1"},
			{Path: "golang.org/x/sys", Version: "v0.15.0", Indirect: true},
		},
		Direct:   1,
		Indirect: 1,
		Total:    2,
	}

	output := FormatScanResult(result, false)
	if !strings.Contains(output, "example.com/test") {
		t.Errorf("expected module path in output: %s", output)
	}
	if !strings.Contains(output, "v0.9.1") {
		t.Errorf("expected version in output: %s", output)
	}
}
