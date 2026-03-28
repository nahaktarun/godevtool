package environ

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nahaktarun/godevtool/internal/color"
)

// Info holds detected environment information.
type Info struct {
	GoVersion  string            `json:"go_version"`
	OS         string            `json:"os"`
	Arch       string            `json:"arch"`
	NumCPU     int               `json:"num_cpu"`
	Hostname   string            `json:"hostname"`
	PID        int               `json:"pid"`
	WorkDir    string            `json:"work_dir"`
	Executable string            `json:"executable"`
	StartTime  time.Time         `json:"start_time"`
	EnvVars    map[string]string `json:"env_vars,omitempty"`
	BuildInfo  *BuildInfo        `json:"build_info,omitempty"`
}

// BuildInfo holds Go build metadata.
type BuildInfo struct {
	Main        string            `json:"main"`
	GoVersion   string            `json:"go_version"`
	Path        string            `json:"path"`
	VCSRevision string            `json:"vcs_revision,omitempty"`
	VCSTime     string            `json:"vcs_time,omitempty"`
	VCSModified bool              `json:"vcs_modified,omitempty"`
	Settings    map[string]string `json:"settings,omitempty"`
	Deps        int               `json:"deps"` // number of dependencies
}

// Uptime returns duration since start.
func (i Info) Uptime() time.Duration {
	return time.Since(i.StartTime)
}

// UptimeStr returns human-readable uptime.
func (i Info) UptimeStr() string {
	d := i.Uptime()
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-d.Minutes()*60)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// defaultEnvFilter returns true for commonly useful environment variables.
func defaultEnvFilter(key string) bool {
	safe := []string{
		"GOPATH", "GOROOT", "GOBIN", "GOPROXY", "GONOSUMCHECK",
		"HOME", "USER", "LOGNAME", "SHELL", "TERM",
		"PATH", "LANG", "LC_ALL",
		"PORT", "HOST", "ADDR",
		"ENV", "APP_ENV", "GO_ENV", "NODE_ENV",
		"DEBUG", "VERBOSE", "LOG_LEVEL",
		"TZ", "HOSTNAME",
	}
	upper := strings.ToUpper(key)
	for _, s := range safe {
		if upper == s {
			return true
		}
	}
	// include anything with common prefixes
	for _, prefix := range []string{"GO", "APP_", "SERVICE_"} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

// Detect collects all environment info. Pass nil for envFilter to use defaults.
func Detect(envFilter func(key string) bool) Info {
	if envFilter == nil {
		envFilter = defaultEnvFilter
	}

	hostname, _ := os.Hostname()
	workDir, _ := os.Getwd()
	executable, _ := os.Executable()

	info := Info{
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		NumCPU:     runtime.NumCPU(),
		Hostname:   hostname,
		PID:        os.Getpid(),
		WorkDir:    workDir,
		Executable: executable,
		StartTime:  time.Now(),
	}

	// collect filtered env vars
	info.EnvVars = make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && envFilter(parts[0]) {
			info.EnvVars[parts[0]] = parts[1]
		}
	}

	// collect build info
	if bi, ok := debug.ReadBuildInfo(); ok {
		binfo := &BuildInfo{
			GoVersion: bi.GoVersion,
			Path:      bi.Path,
			Settings:  make(map[string]string),
			Deps:      len(bi.Deps),
		}
		if bi.Main.Path != "" {
			binfo.Main = bi.Main.Path
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				binfo.VCSRevision = s.Value
			case "vcs.time":
				binfo.VCSTime = s.Value
			case "vcs.modified":
				binfo.VCSModified = s.Value == "true"
			default:
				binfo.Settings[s.Key] = s.Value
			}
		}
		info.BuildInfo = binfo
	}

	return info
}

// FormatInfo returns a human-readable string.
func FormatInfo(info Info, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Environment", colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	rows := []struct {
		label string
		value string
	}{
		{"Go Version", info.GoVersion},
		{"OS / Arch", info.OS + "/" + info.Arch},
		{"CPUs", fmt.Sprintf("%d", info.NumCPU)},
		{"Hostname", info.Hostname},
		{"PID", fmt.Sprintf("%d", info.PID)},
		{"Working Dir", info.WorkDir},
		{"Uptime", info.UptimeStr()},
	}

	if info.BuildInfo != nil {
		if info.BuildInfo.VCSRevision != "" {
			rev := info.BuildInfo.VCSRevision
			if len(rev) > 8 {
				rev = rev[:8]
			}
			modified := ""
			if info.BuildInfo.VCSModified {
				modified = " (modified)"
			}
			rows = append(rows, struct {
				label string
				value string
			}{"Git Revision", rev + modified})
		}
		if info.BuildInfo.Main != "" {
			rows = append(rows, struct {
				label string
				value string
			}{"Module", info.BuildInfo.Main})
		}
		rows = append(rows, struct {
			label string
			value string
		}{"Dependencies", fmt.Sprintf("%d modules", info.BuildInfo.Deps)})
	}

	for _, r := range rows {
		b.WriteString(fmt.Sprintf("  %-14s %s\n",
			color.Wrap(r.label, colorize, color.Blue),
			r.value,
		))
	}

	return b.String()
}
