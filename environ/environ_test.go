package environ

import (
	"runtime"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	info := Detect(nil)

	if info.GoVersion == "" {
		t.Error("GoVersion empty")
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
	if info.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
	if info.NumCPU == 0 {
		t.Error("NumCPU = 0")
	}
	if info.PID == 0 {
		t.Error("PID = 0")
	}
	if info.WorkDir == "" {
		t.Error("WorkDir empty")
	}
	if info.StartTime.IsZero() {
		t.Error("StartTime is zero")
	}
}

func TestDetectEnvFilter(t *testing.T) {
	// custom filter that only includes HOME
	info := Detect(func(key string) bool {
		return key == "HOME"
	})

	for k := range info.EnvVars {
		if k != "HOME" {
			t.Errorf("unexpected env var %q", k)
		}
	}
}

func TestDetectDefaultFilter(t *testing.T) {
	info := Detect(nil)
	// should not include sensitive vars like PASSWORD
	for k := range info.EnvVars {
		if strings.Contains(strings.ToUpper(k), "PASSWORD") ||
			strings.Contains(strings.ToUpper(k), "SECRET") ||
			strings.Contains(strings.ToUpper(k), "TOKEN") {
			t.Errorf("sensitive var leaked: %q", k)
		}
	}
}

func TestUptime(t *testing.T) {
	info := Detect(nil)
	uptime := info.Uptime()
	if uptime <= 0 {
		t.Errorf("uptime = %v, expected > 0", uptime)
	}
	s := info.UptimeStr()
	if s == "" {
		t.Error("UptimeStr empty")
	}
}

func TestFormatInfo(t *testing.T) {
	info := Detect(nil)
	output := FormatInfo(info, false)

	checks := []string{"Go Version", "OS / Arch", "CPUs", "PID"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected %q in output:\n%s", check, output)
		}
	}
}

func TestBuildInfo(t *testing.T) {
	info := Detect(nil)
	// Build info may or may not be available depending on how tests are run
	if info.BuildInfo != nil {
		if info.BuildInfo.GoVersion == "" {
			t.Error("BuildInfo.GoVersion empty")
		}
	}
}
