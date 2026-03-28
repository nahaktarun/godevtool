package export

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Snapshot captures the complete application debug state.
type Snapshot struct {
	Timestamp   time.Time       `json:"timestamp"`
	AppName     string          `json:"app_name,omitempty"`
	Environment json.RawMessage `json:"environment,omitempty"`
	Logs        json.RawMessage `json:"logs,omitempty"`
	Requests    json.RawMessage `json:"requests,omitempty"`
	Goroutines  json.RawMessage `json:"goroutines,omitempty"`
	MemStats    json.RawMessage `json:"memstats,omitempty"`
	Timers      json.RawMessage `json:"timers,omitempty"`
	Queries     json.RawMessage `json:"queries,omitempty"`
	Timeline    json.RawMessage `json:"timeline,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
	Errors      json.RawMessage `json:"errors,omitempty"`
	Outgoing    json.RawMessage `json:"outgoing,omitempty"`
	Caches      json.RawMessage `json:"caches,omitempty"`
	RateLimits  json.RawMessage `json:"rate_limits,omitempty"`
	Benchmarks  json.RawMessage `json:"benchmarks,omitempty"`
	Alerts      json.RawMessage `json:"alerts,omitempty"`
}

// DataSource provides data for export. Each function returns JSON-serializable data.
type DataSource struct {
	AppName     string
	Environment func() any
	Logs        func() any
	Requests    func() any
	Goroutines  func() any
	MemStats    func() any
	Timers      func() any
	Queries     func() any
	Timeline    func() any
	Config      func() any
	Errors      func() any
	Outgoing    func() any
	Caches      func() any
	RateLimits  func() any
	Benchmarks  func() any
	Alerts      func() any
}

// Exporter captures and exports debug snapshots.
type Exporter struct {
	source DataSource
}

// New creates an Exporter with the given data source.
func New(source DataSource) *Exporter {
	return &Exporter{source: source}
}

// CaptureJSON returns a full snapshot as JSON bytes.
func (e *Exporter) CaptureJSON() ([]byte, error) {
	snap := e.capture()
	return json.MarshalIndent(snap, "", "  ")
}

// CaptureHTML returns a self-contained HTML report.
func (e *Exporter) CaptureHTML() ([]byte, error) {
	jsonData, err := e.CaptureJSON()
	if err != nil {
		return nil, err
	}
	return generateHTMLReport(jsonData, e.source.AppName)
}

// WriteTo writes the snapshot to a writer in the given format ("json" or "html").
func (e *Exporter) WriteTo(w io.Writer, format string) error {
	var data []byte
	var err error

	switch format {
	case "json":
		data, err = e.CaptureJSON()
	case "html":
		data, err = e.CaptureHTML()
	default:
		return fmt.Errorf("unsupported format: %s (use 'json' or 'html')", format)
	}

	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (e *Exporter) capture() Snapshot {
	snap := Snapshot{
		Timestamp: time.Now(),
		AppName:   e.source.AppName,
	}

	snap.Environment = marshalSafe(e.source.Environment)
	snap.Logs = marshalSafe(e.source.Logs)
	snap.Requests = marshalSafe(e.source.Requests)
	snap.Goroutines = marshalSafe(e.source.Goroutines)
	snap.MemStats = marshalSafe(e.source.MemStats)
	snap.Timers = marshalSafe(e.source.Timers)
	snap.Queries = marshalSafe(e.source.Queries)
	snap.Timeline = marshalSafe(e.source.Timeline)
	snap.Config = marshalSafe(e.source.Config)
	snap.Errors = marshalSafe(e.source.Errors)
	snap.Outgoing = marshalSafe(e.source.Outgoing)
	snap.Caches = marshalSafe(e.source.Caches)
	snap.RateLimits = marshalSafe(e.source.RateLimits)
	snap.Benchmarks = marshalSafe(e.source.Benchmarks)
	snap.Alerts = marshalSafe(e.source.Alerts)

	return snap
}

func marshalSafe(fn func() any) json.RawMessage {
	if fn == nil {
		return nil
	}
	data, err := json.Marshal(fn())
	if err != nil {
		return json.RawMessage(`null`)
	}
	return data
}

func generateHTMLReport(jsonData []byte, appName string) ([]byte, error) {
	title := "godevtool Debug Report"
	if appName != "" {
		title = appName + " — " + title
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
:root { --bg: #0d1117; --bg2: #161b22; --border: #30363d; --text: #c9d1d9; --muted: #8b949e; --accent: #58a6ff; --green: #3fb950; --yellow: #d29922; --red: #f85149; --mono: 'SF Mono', monospace; }
* { margin:0; padding:0; box-sizing:border-box; }
body { font-family: -apple-system, sans-serif; background: var(--bg); color: var(--text); padding: 24px; }
h1 { color: var(--accent); font-size: 20px; margin-bottom: 8px; }
.meta { color: var(--muted); font-size: 13px; margin-bottom: 24px; }
.section { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; margin-bottom: 16px; padding: 16px; }
.section h2 { color: var(--accent); font-size: 15px; margin-bottom: 12px; cursor: pointer; }
.section h2:hover { text-decoration: underline; }
pre { background: var(--bg); border: 1px solid var(--border); border-radius: 6px; padding: 12px; overflow-x: auto; font-family: var(--mono); font-size: 12px; color: var(--text); max-height: 400px; overflow-y: auto; }
.collapsed pre { display: none; }
table { width: 100%%; border-collapse: collapse; font-size: 13px; font-family: var(--mono); }
th { text-align: left; padding: 6px 10px; color: var(--muted); font-size: 11px; text-transform: uppercase; border-bottom: 1px solid var(--border); }
td { padding: 6px 10px; border-bottom: 1px solid var(--border); }
</style>
</head>
<body>
<h1>%s</h1>
<div class="meta">Captured at %s</div>
<div id="sections"></div>
<script>
const data = %s;
const sections = document.getElementById('sections');
const keys = Object.keys(data).filter(k => k !== 'timestamp' && k !== 'app_name' && data[k] !== null);
keys.forEach(key => {
  const div = document.createElement('div');
  div.className = 'section';
  const title = key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
  const content = JSON.stringify(data[key], null, 2);
  div.innerHTML = '<h2 onclick="this.parentElement.classList.toggle(\'collapsed\')">' + title + '</h2><pre>' + escapeHtml(content) + '</pre>';
  sections.appendChild(div);
});
function escapeHtml(s) { return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
</script>
</body>
</html>`, title, title, time.Now().Format("2006-01-02 15:04:05"), string(jsonData))

	return []byte(html), nil
}
