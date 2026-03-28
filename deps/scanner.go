package deps

import (
	"bufio"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/nahaktarun/godevtool/internal/color"
)

// Module represents a single dependency.
type Module struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Indirect bool   `json:"indirect"`
	Replace  string `json:"replace,omitempty"`
}

// ScanResult holds dependency scan results.
type ScanResult struct {
	GoVersion  string   `json:"go_version"`
	ModulePath string   `json:"module_path"`
	Modules    []Module `json:"modules"`
	Direct     int      `json:"direct"`
	Indirect   int      `json:"indirect"`
	Total      int      `json:"total"`
}

// ScanFromBuildInfo extracts deps from runtime/debug.ReadBuildInfo().
// This works without filesystem access and is the primary method.
func ScanFromBuildInfo() (ScanResult, error) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ScanResult{}, fmt.Errorf("build info not available")
	}

	result := ScanResult{
		GoVersion:  bi.GoVersion,
		ModulePath: bi.Main.Path,
	}

	for _, dep := range bi.Deps {
		m := Module{
			Path:    dep.Path,
			Version: dep.Version,
		}
		if dep.Replace != nil {
			m.Replace = dep.Replace.Path + "@" + dep.Replace.Version
		}
		// heuristic: if the module is not in the main module's import graph
		// it's typically indirect, but ReadBuildInfo doesn't distinguish
		result.Modules = append(result.Modules, m)
	}

	result.Total = len(result.Modules)
	// ReadBuildInfo doesn't distinguish direct vs indirect in Deps
	// All deps from ReadBuildInfo are in the build graph
	result.Direct = result.Total

	return result, nil
}

// ScanGoMod reads and parses a go.mod file at the given path.
func ScanGoMod(path string) (ScanResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return ScanResult{}, fmt.Errorf("open go.mod: %w", err)
	}
	defer f.Close()

	var result ScanResult
	scanner := bufio.NewScanner(f)
	inRequire := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// module declaration
		if strings.HasPrefix(line, "module ") {
			result.ModulePath = strings.TrimPrefix(line, "module ")
			result.ModulePath = strings.TrimSpace(result.ModulePath)
			continue
		}

		// go version
		if strings.HasPrefix(line, "go ") {
			result.GoVersion = strings.TrimPrefix(line, "go ")
			result.GoVersion = strings.TrimSpace(result.GoVersion)
			continue
		}

		// require block
		if line == "require (" {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}

		// single-line require
		if strings.HasPrefix(line, "require ") {
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				m := Module{Path: parts[0], Version: parts[1]}
				result.Modules = append(result.Modules, m)
				result.Direct++
			}
			continue
		}

		// inside require block
		if inRequire {
			indirect := strings.Contains(line, "// indirect")
			line = strings.Split(line, "//")[0]
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) >= 2 {
				m := Module{Path: parts[0], Version: parts[1], Indirect: indirect}
				result.Modules = append(result.Modules, m)
				if indirect {
					result.Indirect++
				} else {
					result.Direct++
				}
			}
		}
	}

	result.Total = len(result.Modules)
	return result, scanner.Err()
}

// FormatScanResult returns a human-readable string.
func FormatScanResult(r ScanResult, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Dependencies", colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	b.WriteString(fmt.Sprintf("  %-14s %s\n",
		color.Wrap("Module", colorize, color.Blue), r.ModulePath))
	b.WriteString(fmt.Sprintf("  %-14s %s\n",
		color.Wrap("Go Version", colorize, color.Blue), r.GoVersion))
	b.WriteString(fmt.Sprintf("  %-14s %d direct, %d indirect (%d total)\n",
		color.Wrap("Dependencies", colorize, color.Blue),
		r.Direct, r.Indirect, r.Total))

	if len(r.Modules) > 0 {
		b.WriteByte('\n')

		maxPath := 6
		for _, m := range r.Modules {
			if len(m.Path) > maxPath {
				maxPath = len(m.Path)
			}
		}
		if maxPath > 50 {
			maxPath = 50
		}

		for _, m := range r.Modules {
			path := m.Path
			if len(path) > 50 {
				path = path[:47] + "..."
			}
			indirect := ""
			if m.Indirect {
				indirect = color.Wrap(" (indirect)", colorize, color.Gray)
			}
			b.WriteString(fmt.Sprintf("  %-*s %s%s\n",
				maxPath, path,
				color.Wrap(m.Version, colorize, color.Green),
				indirect))
		}
	}

	return b.String()
}
