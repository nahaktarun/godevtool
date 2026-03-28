// Package godevtool is a comprehensive developer debugging toolkit for Go applications.
//
// It provides structured logging, variable inspection, performance profiling, HTTP
// request/response monitoring, goroutine tracking, memory analysis, database query
// logging, event timelines, configuration viewing, error tracking, and a real-time
// web dashboard -- all with zero external dependencies.
//
// # Quick Start
//
//	dt := godevtool.New(
//	    godevtool.WithAppName("myapp"),
//	    godevtool.WithLogLevel(godevtool.LevelDebug),
//	)
//	defer dt.Shutdown()
//
//	// Structured logging
//	dt.Log.Info("server starting", "port", 8080)
//
//	// Variable inspection
//	dt.Inspect(myStruct)
//
//	// Execution timing
//	defer dt.Timer("db-query").Stop()
//
//	// Web dashboard
//	dt.StartDashboard(":9999")
//
// # Architecture
//
// The DevTool struct is the central facade that wires together 26 specialized packages.
// Each package can be used independently, but the facade provides a unified API and
// connects everything to the web dashboard for real-time visualization.
//
// All features are designed to be zero-overhead when disabled via [DevTool.Disable].
//
// # Packages
//
// Core debugging:
//   - [log] - Structured, colorized logging with key-value pairs
//   - [inspect] - Pretty-print any Go value with type information
//   - [timer] - Execution timing with aggregate statistics
//   - [stack] - Stack trace capture and formatting
//
// Runtime monitoring:
//   - [middleware] - HTTP request/response capture
//   - [goroutine] - Goroutine count tracking and leak detection
//   - [memstats] - Memory and GC statistics collection
//
// Advanced tracing:
//   - [dblog] - Database query logging with sql.DB wrapping
//   - [timeline] - Event timeline with point-in-time events and spans
//   - [config] - Configuration viewer with field redaction
//
// Environment awareness:
//   - [environ] - Go version, OS, build info, hostname detection
//   - [deps] - Module dependency scanning
//   - [errtrack] - Error tracking, grouping, and panic recovery
//   - [profiler] - On-demand pprof profile capture
//
// Performance intelligence:
//   - [httptrace] - Outgoing HTTP request tracing with timing breakdown
//   - [cachemon] - Cache hit/miss/eviction monitoring
//   - [ratelimit] - Rate limiter decision tracking
//   - [bench] - Micro-benchmarking with percentile statistics
//
// Developer experience:
//   - [alerts] - Configurable threshold alert rules
//   - [export] - Debug session export as JSON or HTML report
//   - [grpcmon] - gRPC call monitoring
//   - [hotreload] - File watching with auto-rebuild
//   - [dashboard] - Real-time web dashboard with SSE
package godevtool
