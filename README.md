# godevtool

A comprehensive developer debugging toolkit for Go applications.

**Zero external dependencies. 26 packages. 180 tests. Real-time web dashboard.**

## Features

| Category | Features |
|----------|----------|
| **Logging** | Structured, colorized logger with levels, key-value pairs, child loggers |
| **Inspection** | Pretty-print any Go value with type info, nested structs, slices, maps |
| **Timing** | Execution timer with defer pattern, aggregate stats (min/max/avg) |
| **Stack Traces** | Capture and format stack traces with runtime frame filtering |
| **HTTP Middleware** | Capture incoming request/response details, headers, bodies, timing |
| **Goroutines** | Monitor count, detect leaks, view goroutine state |
| **Memory** | Track heap, GC cycles, allocation rates |
| **Database** | Log SQL queries with timing, args, caller info; wraps `*sql.DB` |
| **Timeline** | Record events and spans across your application lifecycle |
| **Config Viewer** | Display configuration with automatic secret redaction |
| **Error Tracking** | Group errors, track rates (1m/5m/15m), panic recovery middleware |
| **Profiler** | On-demand CPU/heap/goroutine/mutex profiles with download |
| **HTTP Tracing** | Trace outgoing HTTP with DNS/TCP/TLS/server timing breakdown |
| **Cache Monitor** | Track hit/miss/eviction rates across named caches |
| **Rate Limits** | Monitor rate limiter decisions (allowed/throttled/queued) |
| **Benchmarks** | Micro-benchmark runner with P50/P90/P99 percentiles |
| **Alerts** | Threshold-based alert rules with firing/resolved state machine |
| **Export** | Export debug snapshots as JSON or self-contained HTML reports |
| **gRPC** | Monitor gRPC calls (unary/stream, server/client) |
| **Hot Reload** | File watcher with auto-rebuild on `.go` file changes |
| **Dashboard** | Real-time web UI with 19 tabs, SSE streaming, dark theme |

## Installation

```bash
go get github.com/tarunnahak/godevtool
```

Requires Go 1.21 or later.

## Quick Start

```go
package main

import (
    "github.com/tarunnahak/godevtool"
    "github.com/tarunnahak/godevtool/log"
)

func main() {
    dt := godevtool.New(
        godevtool.WithAppName("myapp"),
        godevtool.WithLogLevel(log.LevelDebug),
    )
    defer dt.Shutdown()

    // Structured logging
    dt.Log.Info("server starting", "port", 8080, "env", "development")

    // Inspect any value
    dt.Inspect(myStruct)

    // Time operations
    defer dt.Timer("database-query").Stop()

    // Start the web dashboard
    dt.StartDashboard(":9999")
    // Open http://localhost:9999 in your browser
}
```

## Usage Guide

### Logging

```go
dt := godevtool.New(godevtool.WithAppName("api"))

dt.Log.Info("request received", "method", "GET", "path", "/users")
dt.Log.Warn("cache miss", "key", "user:42")
dt.Log.Error("query failed", err, "table", "users")
dt.Log.Debug("parsed payload", "size", 1024)

// Child logger with persistent context
reqLog := dt.Log.With("request_id", "abc-123", "user_id", 42)
reqLog.Info("processing")
```

Output:
```
15:04:05.123 INFO  [api] request received              method=GET path=/users
15:04:05.124 WARN  [api] cache miss                    key=user:42
15:04:05.125 ERROR [api] query failed                  error="connection refused" table=users
```

### Variable Inspector

```go
type User struct {
    Name    string
    Email   string
    Roles   []string
    Address struct {
        City  string
        State string
    }
}

dt.Inspect(User{
    Name:  "Alice",
    Email: "alice@example.com",
    Roles: []string{"admin", "editor"},
    Address: struct{ City, State string }{"Springfield", "IL"},
})
```

Output:
```
(main.User) {
  Name   : (string) "Alice"
  Email  : (string) "alice@example.com"
  Roles  : ([]string) [2 items] [
    [0] (string) "admin"
    [1] (string) "editor"
  ]
  Address: (struct) {
    City : (string) "Springfield"
    State: (string) "IL"
  }
}
```

### HTTP Middleware

```go
mux := http.NewServeMux()
mux.HandleFunc("/api/users", handleUsers)

// Wrap with devtool middleware - captures all requests
handler := dt.Middleware().Handler(mux)

// Add panic recovery
handler = dt.RecoverMiddleware(handler)

http.ListenAndServe(":8080", handler)
```

### Execution Timer

```go
// Simple timing with defer
func processOrder(id string) {
    defer dt.Timer("process-order").Stop()
    // ... work ...
}

// View aggregate stats
dt.PrintTimerReport()
// Output:
// LABEL            COUNT   TOTAL         AVG          MIN          MAX
// process-order        5   250ms         50ms         30ms         80ms
```

### Web Dashboard

```go
// Start monitors
dt.StartGoroutineMonitor(3 * time.Second)
dt.StartMemStats(3 * time.Second)

// Launch dashboard on a separate port
dt.StartDashboard(":9999")
// Open http://localhost:9999
```

The dashboard provides 19 real-time tabs: Overview, Logs, Requests, Goroutines, Memory, Timers, Queries, Timeline, Config, Errors, Environment, Dependencies, Profiler, Outgoing HTTP, Caches, Rate Limits, Benchmarks, Alerts, gRPC.

### Database Query Logging

```go
import "github.com/tarunnahak/godevtool/dblog"

// Wrap your *sql.DB
wrappedDB := dblog.WrapDB(db, dt.DBLogger())

// All queries are now automatically logged
rows, err := wrappedDB.QueryContext(ctx, "SELECT * FROM users WHERE id = $1", 42)
```

### Event Timeline

```go
import "github.com/tarunnahak/godevtool/timeline"

// Point-in-time events
dt.TimelineRecord(timeline.CatHTTP, "GET /api/users", map[string]any{"status": 200})

// Spans (events with duration)
span := dt.TimelineStart(timeline.CatDB, "SELECT * FROM users", nil)
// ... do work ...
span.SetData("rows", 15)
span.End()
```

### Configuration Viewer

```go
type Config struct {
    Host     string `json:"host" env:"APP_HOST"`
    Port     int    `json:"port"`
    DBPass   string `devtool:"redact"` // automatically masked
    APIKey   string `devtool:"redact"` // automatically masked
}

dt.RegisterConfig("app", Config{
    Host:   "localhost",
    Port:   8080,
    DBPass: "secret123",
    APIKey: "sk-prod-xxxxx",
})

dt.PrintConfig()
// Output:
// app
//   Host     localhost  (string)  [env:APP_HOST]
//   Port     8080       (int)    [json:port]
//   DBPass   ********   (string)
//   APIKey   ********   (string)
```

### Error Tracking

```go
// Track errors
dt.TrackError(err, map[string]any{"endpoint": "/api/users"})

// Panic recovery middleware
handler = dt.RecoverMiddleware(handler)

// View stats
dt.PrintErrorStats()
// Groups similar errors, shows rates for last 1m/5m/15m
```

### Profiling

```go
// Capture heap snapshot
prof, _ := dt.CaptureHeapProfile()

// Capture CPU profile (blocking for duration)
prof, _ := dt.CaptureCPUProfile(30 * time.Second)

// Profiles are downloadable from the dashboard
```

### HTTP Client Tracing

```go
// Wrap any http.Client
client := dt.WrapHTTPClient(&http.Client{Timeout: 10 * time.Second})

// All requests are traced with timing breakdown
resp, err := client.Get("https://api.example.com/data")
// Captures: DNS lookup, TCP connect, TLS handshake, server processing, content transfer
```

### Cache Monitoring

```go
cache := dt.RegisterCache("users")

// In your cache implementation:
if value, ok := myCache.Get(key); ok {
    cache.Hit()
} else {
    cache.Miss()
}
cache.SetSize(int64(myCache.Len()))
```

### Benchmarking

```go
result := dt.Benchmark("json-marshal", 10000, func() {
    json.Marshal(data)
})
// result.AvgTime, result.P50, result.P90, result.P99, result.OpsPerSec
```

### Alert Rules

```go
// Built-in rules
dt.AlertOnGoroutineCount(100)            // warn if > 100 goroutines
dt.AlertOnHeapAlloc(512 * 1024 * 1024)   // warn if heap > 512MB

// Custom rules
dt.AddAlertRule(alerts.CustomRule(
    "error_rate", alerts.SeverityCritical, 10,
    "error rate exceeds threshold",
    func() (float64, bool) {
        stats := dt.ErrorTracker().Stats()
        return float64(stats.Last5Min), stats.Last5Min > 10
    },
))

// Start evaluation
dt.StartAlerts(10 * time.Second)
```

### Export Debug Sessions

```go
// Export as JSON
data, _ := dt.ExportJSON()

// Export as self-contained HTML report
data, _ := dt.ExportHTML()

// Write to file
dt.ExportToFile("debug-report.html", "html")

// Also available from the dashboard via Export buttons
```

### Hot Reload

```go
import "github.com/tarunnahak/godevtool/hotreload"

dt.StartHotReload(
    hotreload.WithDirs("."),
    hotreload.WithBuildCmd("go build -o ./tmp/main ."),
    hotreload.WithDebounce(500 * time.Millisecond),
)
```

### Production Safety

```go
dt := godevtool.New()

// Disable in production - all methods become no-ops
if os.Getenv("ENV") == "production" {
    dt.Disable()
}
```

## Configuration Options

```go
dt := godevtool.New(
    godevtool.WithAppName("myapp"),           // prefix for log lines
    godevtool.WithLogLevel(log.LevelDebug),   // min log level
    godevtool.WithOutput(os.Stderr),          // output destination
    godevtool.WithNoColor(),                  // disable ANSI colors
    godevtool.WithTimeFormat("2006-01-02 15:04:05"), // timestamp format
    godevtool.WithMaxDepth(15),               // inspect recursion depth
)
```

## Package Architecture

```
godevtool/
├── godevtool.go           Main DevTool facade (57 public methods)
├── options.go             Configuration options
├── internal/
│   ├── color/             ANSI color helpers
│   └── ringbuf/           Generic thread-safe ring buffer
├── log/                   Structured pretty logger
├── inspect/               Variable/struct inspector
├── timer/                 Execution timer + reports
├── stack/                 Stack trace prettifier
├── middleware/             HTTP request/response capture
├── goroutine/             Goroutine monitor + leak detection
├── memstats/              Memory/GC stats collector
├── dashboard/             Web dashboard + SSE + REST API
│   └── static/            Embedded HTML/JS/CSS
├── dblog/                 Database query logger
├── timeline/              Event timeline with spans
├── config/                Config viewer with redaction
├── environ/               Environment detector
├── deps/                  Dependency scanner
├── errtrack/              Error tracker + panic recovery
├── profiler/              Pprof integration
├── httptrace/             HTTP client tracer
├── cachemon/              Cache monitor
├── ratelimit/             Rate limiter monitor
├── bench/                 Benchmark runner
├── alerts/                Alert rules engine
├── export/                JSON/HTML export
├── grpcmon/               gRPC call monitor
├── hotreload/             File watcher + auto-rebuild
└── examples/
    ├── basic/             CLI-only demo
    ├── http-server/       HTTP server with debug endpoints
    └── full-dashboard/    Full demo with all features
```

## Examples

### Basic (CLI only)
```bash
go run examples/basic/main.go
```

### HTTP Server with Debug Endpoints
```bash
go run examples/http-server/main.go
# http://localhost:8080/api/users
# http://localhost:8080/debug/requests
```

### Full Dashboard
```bash
go run examples/full-dashboard/main.go
# App:       http://localhost:8080
# Dashboard: http://localhost:9999
```

## Design Principles

- **Zero dependencies** - Pure Go standard library only
- **Zero overhead when disabled** - `Disable()` makes all methods no-ops
- **No global state** - Everything hangs off the `DevTool` instance
- **Bounded memory** - Ring buffers prevent unbounded growth
- **Thread-safe** - All types are safe for concurrent use
- **Functional options** - Idiomatic Go configuration pattern

## API Reference

Full API documentation: [pkg.go.dev/github.com/tarunnahak/godevtool](https://pkg.go.dev/github.com/tarunnahak/godevtool)

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Run tests (`go test ./...`)
4. Commit your changes
5. Push to the branch
6. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.
