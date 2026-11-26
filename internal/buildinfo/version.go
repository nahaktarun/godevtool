// Package buildinfo exposes the godevtool module version embedded in Go build metadata.
package buildinfo

import "runtime/debug"

const modulePath = "github.com/nahaktarun/godevtool"

// Version returns the godevtool dependency version, or "(devel)" when the
// application is built from this module or build metadata is unavailable.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(devel)"
	}
	return versionFromBuildInfo(info)
}

func versionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil || info.Main.Path == modulePath {
		return "(devel)"
	}
	for _, dep := range info.Deps {
		if dep.Path == modulePath && dep.Version != "" {
			return dep.Version
		}
	}
	return "(devel)"
}
