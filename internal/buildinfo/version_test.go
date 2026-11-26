package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{name: "missing build info", info: nil, want: "(devel)"},
		{name: "main module", info: &debug.BuildInfo{Main: debug.Module{Path: modulePath}}, want: "(devel)"},
		{name: "missing dependency", info: &debug.BuildInfo{Main: debug.Module{Path: "example.com/app"}}, want: "(devel)"},
		{name: "empty dependency version", info: &debug.BuildInfo{Main: debug.Module{Path: "example.com/app"}, Deps: []*debug.Module{{Path: modulePath}}}, want: "(devel)"},
		{name: "dependency version", info: &debug.BuildInfo{Main: debug.Module{Path: "example.com/app"}, Deps: []*debug.Module{{Path: modulePath, Version: "v1.2.3"}}}, want: "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionFromBuildInfo(tt.info); got != tt.want {
				t.Fatalf("versionFromBuildInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionDoesNotReturnEmpty(t *testing.T) {
	if got := Version(); got == "" {
		t.Fatal("Version() returned an empty string")
	}
}
