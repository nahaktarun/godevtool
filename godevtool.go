package godevtool

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/nahaktarun/godevtool/alerts"
	"github.com/nahaktarun/godevtool/bench"
	"github.com/nahaktarun/godevtool/cachemon"
	"github.com/nahaktarun/godevtool/config"
	"github.com/nahaktarun/godevtool/dashboard"
	"github.com/nahaktarun/godevtool/dblog"
	"github.com/nahaktarun/godevtool/deps"
	"github.com/nahaktarun/godevtool/environ"
	"github.com/nahaktarun/godevtool/errtrack"
	"github.com/nahaktarun/godevtool/export"
	"github.com/nahaktarun/godevtool/grpcmon"
	"github.com/nahaktarun/godevtool/goroutine"
	"github.com/nahaktarun/godevtool/hotreload"
	"github.com/nahaktarun/godevtool/httptrace"
	"github.com/nahaktarun/godevtool/inspect"
	"github.com/nahaktarun/godevtool/internal/color"
	"github.com/nahaktarun/godevtool/log"
	"github.com/nahaktarun/godevtool/memstats"
	"github.com/nahaktarun/godevtool/middleware"
	"github.com/nahaktarun/godevtool/profiler"
	"github.com/nahaktarun/godevtool/ratelimit"
	"github.com/nahaktarun/godevtool/stack"
	"github.com/nahaktarun/godevtool/timeline"
	"github.com/nahaktarun/godevtool/timer"
)

// Re-export log levels for convenience.
const (
	LevelDebug = log.LevelDebug
	LevelInfo  = log.LevelInfo
	LevelWarn  = log.LevelWarn
	LevelError = log.LevelError
)

// DevTool is the central handle for all debugging facilities.
// It is safe for concurrent use.
type DevTool struct {
	Log     *log.Logger
	opts    options
	output  io.Writer
	enabled bool
	report  *timer.Report

	// Phase 2
	inspector  *middleware.Inspector
	goroutines *goroutine.Monitor
	memstats   *memstats.Collector

	// Phase 3
	dashboard *dashboard.Server

	// Phase 4
	dbLogger   *dblog.Logger
	timeline   *timeline.Timeline
	configView *config.Viewer

	// Phase 5
	envInfo    environ.Info
	depsResult *deps.ScanResult
	errTracker *errtrack.Tracker
	prof       *profiler.Profiler

	// Phase 6
	httpTracer  *httptrace.Tracer
	cacheMon    *cachemon.Monitor
	rateMon     *ratelimit.Monitor
	benchRunner *bench.Runner

	// Phase 7
	alertEngine *alerts.Engine
	exporter    *export.Exporter
	grpcMon     *grpcmon.Monitor
	hotReload   *hotreload.Watcher
}

// New creates a DevTool instance. Pass Option values to customize behavior.
func New(opts ...Option) *DevTool {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}

	out := o.output
	if out == nil {
		out = os.Stdout
	}

	// auto-detect color
	colorize := true
	if o.colorize != nil {
		colorize = *o.colorize
	} else if f, ok := out.(*os.File); ok {
		colorize = color.IsTerminal(f)
	}

	logger := log.New(out, o.logLevel, colorize, o.timeFormat)
	if o.appName != "" {
		logger = logger.WithPrefix(o.appName)
	}

	dt := &DevTool{
		Log:     logger,
		opts:    o,
		output:  out,
		enabled: true,
		report:  timer.NewReport(),
	}

	// Initialize middleware inspector with logging callback
	dt.inspector = middleware.New(
		middleware.WithOnLog(func(rl middleware.RequestLog) {
			dt.Log.Debug("http request",
				"method", rl.Method,
				"path", rl.Path,
				"status", rl.StatusCode,
				"duration", rl.Duration,
			)
		}),
	)

	// Phase 4: Initialize DB logger, timeline, config viewer
	dt.timeline = timeline.New(
		timeline.WithOnEvent(func(evt timeline.Event) {
			if dt.dashboard != nil {
				dt.dashboard.Hub().Broadcast(dashboard.Event{
					Type: "timeline",
					Data: evt,
				})
			}
		}),
	)
	dt.dbLogger = dblog.New(
		dblog.WithOnLog(func(ql dblog.QueryLog) {
			dt.Log.Debug("db query",
				"op", ql.Operation,
				"query", ql.Query,
				"duration", ql.Duration,
			)
			dt.timeline.Record(timeline.CatDB, ql.Query, map[string]any{
				"duration": ql.Duration.String(),
				"rows":     ql.Rows,
			})
		}),
	)
	dt.configView = config.New()

	// Phase 5: Environment, deps, error tracker, profiler
	dt.envInfo = environ.Detect(nil)
	dt.errTracker = errtrack.New(
		errtrack.WithOnError(func(te errtrack.TrackedError) {
			if dt.dashboard != nil {
				dt.dashboard.Hub().Broadcast(dashboard.Event{
					Type: "error",
					Data: te,
				})
			}
			dt.timeline.Record(timeline.CatCustom, "error: "+te.Message, map[string]any{
				"type": te.Type,
			})
		}),
	)
	dt.prof = profiler.New()

	// Phase 6: HTTP tracer, cache monitor, rate limiter monitor, benchmark
	dt.httpTracer = httptrace.New(
		httptrace.WithOnTrace(func(rt httptrace.RequestTrace) {
			dt.Log.Debug("outgoing http",
				"method", rt.Method,
				"url", rt.URL,
				"status", rt.StatusCode,
				"duration", rt.Duration,
			)
			if dt.dashboard != nil {
				dt.dashboard.Hub().Broadcast(dashboard.Event{
					Type: "outgoing",
					Data: rt,
				})
			}
		}),
	)
	dt.cacheMon = cachemon.New()
	dt.rateMon = ratelimit.New()
	dt.benchRunner = bench.New(
		bench.WithOnResult(func(r bench.Result) {
			dt.Log.Debug("benchmark complete",
				"label", r.Label,
				"iterations", r.Iterations,
				"avg", r.AvgTime,
				"p99", r.P99,
			)
		}),
	)

	// Phase 7: Alerts, export, gRPC monitor
	dt.alertEngine = alerts.New(
		alerts.WithOnAlert(func(a alerts.Alert) {
			if a.State == alerts.StateFiring {
				dt.Log.Warn("alert firing", "rule", a.RuleName, "value", a.Value, "severity", a.Severity)
			} else {
				dt.Log.Info("alert resolved", "rule", a.RuleName)
			}
			if dt.dashboard != nil {
				dt.dashboard.Hub().Broadcast(dashboard.Event{
					Type: "alert",
					Data: a,
				})
			}
		}),
	)
	dt.grpcMon = grpcmon.New(
		grpcmon.WithOnCall(func(cl grpcmon.CallLog) {
			dt.Log.Debug("grpc call",
				"method", cl.Method,
				"duration", cl.Duration,
				"type", cl.Type,
			)
			if dt.dashboard != nil {
				dt.dashboard.Hub().Broadcast(dashboard.Event{
					Type: "grpc",
					Data: cl,
				})
			}
		}),
	)

	return dt
}

// Inspect pretty-prints any Go value to the configured output.
// Returns the formatted string.
func (d *DevTool) Inspect(v any) string {
	if !d.enabled {
		return ""
	}
	cfg := inspect.Config{
		MaxDepth:    d.opts.maxDepth,
		Colorize:    d.isColorized(),
		ShowPrivate: true,
		Output:      d.output,
	}
	s := inspect.Sprint(v, cfg)
	fmt.Fprintln(d.output, s)
	return s
}

// InspectTo writes the inspection output to w.
func (d *DevTool) InspectTo(w io.Writer, v any) string {
	if !d.enabled {
		return ""
	}
	cfg := inspect.Config{
		MaxDepth:    d.opts.maxDepth,
		Colorize:    false, // no color when writing to arbitrary writer
		ShowPrivate: true,
		Output:      w,
	}
	s := inspect.Sprint(v, cfg)
	fmt.Fprintln(w, s)
	return s
}

// Timer returns a started Timer. Call Stop() (typically via defer) to
// record elapsed time.
//
//	defer d.Timer("db-query").Stop()
func (d *DevTool) Timer(label string) *timer.Timer {
	if !d.enabled {
		return timer.Start(label, nil)
	}
	return timer.Start(label, func(l string, elapsed time.Duration) {
		d.report.Record(l, elapsed)
		d.Log.Debug("timer", "label", l, "elapsed", elapsed)
	})
}

// TimerReport returns the aggregate timing report.
func (d *DevTool) TimerReport() *timer.Report {
	return d.report
}

// PrintTimerReport writes the timing report table to the configured output.
func (d *DevTool) PrintTimerReport() {
	if !d.enabled {
		return
	}
	d.report.PrintTo(d.output)
}

// Stack returns a prettified stack trace string starting from the caller.
func (d *DevTool) Stack(skip int) string {
	if !d.enabled {
		return ""
	}
	t := stack.Capture(skip + 1)
	cfg := stack.Config{
		Colorize:      d.isColorized(),
		FilterRuntime: true,
	}
	return t.Format(cfg)
}

// PrintStack prints a prettified stack trace to the configured output.
func (d *DevTool) PrintStack() {
	if !d.enabled {
		return
	}
	s := d.Stack(1)
	fmt.Fprint(d.output, s)
}

// Disable turns off all output. All methods become no-ops.
func (d *DevTool) Disable() {
	d.enabled = false
	d.Log.SetEnabled(false)
}

// Enable re-enables output after Disable().
func (d *DevTool) Enable() {
	d.enabled = true
	d.Log.SetEnabled(true)
}

// --- Phase 2: HTTP Middleware, Goroutine Monitor, MemStats ---

// Middleware returns the HTTP request/response inspector.
func (d *DevTool) Middleware() *middleware.Inspector {
	return d.inspector
}

// StartGoroutineMonitor begins tracking goroutines at the given interval.
func (d *DevTool) StartGoroutineMonitor(interval time.Duration) {
	if !d.enabled {
		return
	}
	d.goroutines = goroutine.NewMonitor(interval)
	d.goroutines.Start()
	d.Log.Debug("goroutine monitor started", "interval", interval)
}

// StopGoroutineMonitor stops goroutine tracking.
func (d *DevTool) StopGoroutineMonitor() {
	if d.goroutines != nil {
		d.goroutines.Stop()
	}
}

// GoroutineSnapshot returns the current goroutine snapshot.
func (d *DevTool) GoroutineSnapshot() goroutine.Snapshot {
	if d.goroutines == nil {
		return goroutine.Snapshot{}
	}
	return d.goroutines.Current()
}

// PrintGoroutines prints the current goroutine state to output.
func (d *DevTool) PrintGoroutines() {
	if !d.enabled {
		return
	}
	var snap goroutine.Snapshot
	if d.goroutines != nil {
		snap = d.goroutines.Current()
	} else {
		// one-shot snapshot without starting the monitor
		m := goroutine.NewMonitor(0)
		snap = m.Current()
	}
	fmt.Fprint(d.output, goroutine.FormatSnapshot(snap, d.isColorized()))
}

// GoroutineLeakCheck returns suspected goroutine leaks.
func (d *DevTool) GoroutineLeakCheck() []goroutine.GoroutineInfo {
	if d.goroutines == nil {
		return nil
	}
	return d.goroutines.LeakCheck()
}

// StartMemStats begins collecting memory/GC statistics at the given interval.
func (d *DevTool) StartMemStats(interval time.Duration) {
	if !d.enabled {
		return
	}
	d.memstats = memstats.NewCollector(interval, 100)
	d.memstats.Start()
	d.Log.Debug("memstats collector started", "interval", interval)
}

// StopMemStats stops memory statistics collection.
func (d *DevTool) StopMemStats() {
	if d.memstats != nil {
		d.memstats.Stop()
	}
}

// MemSnapshot returns the current memory snapshot.
func (d *DevTool) MemSnapshot() memstats.Snapshot {
	if d.memstats != nil {
		return d.memstats.Current()
	}
	// one-shot if collector not started
	c := memstats.NewCollector(0, 1)
	return c.Current()
}

// PrintMemStats prints the current memory statistics to output.
func (d *DevTool) PrintMemStats() {
	if !d.enabled {
		return
	}
	snap := d.MemSnapshot()
	memstats.PrintSnapshot(d.output, snap, d.isColorized())
}

// --- Phase 3: Web Dashboard ---

// StartDashboard starts the web dashboard on the given address (e.g. ":9999").
// It wires all subsystems into the dashboard for real-time visualization.
func (d *DevTool) StartDashboard(addr string) error {
	if !d.enabled {
		return nil
	}

	providers := dashboard.DataProviders{
		Logger:     d.Log,
		Middleware: d.inspector,
		Goroutine: func() goroutine.Snapshot {
			return d.GoroutineSnapshot()
		},
		MemStats: func() memstats.Snapshot {
			return d.MemSnapshot()
		},
		Timer:    d.report,
		DBLogger: d.dbLogger,
		Timeline: d.timeline,
		Config:   d.configView,
		// Phase 5
		Environ: func() environ.Info {
			info := d.envInfo
			return info
		},
		Deps: func() deps.ScanResult {
			return d.Dependencies()
		},
		ErrTracker: d.errTracker,
		Profiler:   d.prof,
		// Phase 6
		HTTPTracer: d.httpTracer,
		CacheMon:   d.cacheMon,
		RateMon:    d.rateMon,
		Bench:      d.benchRunner,
		// Phase 7
		Alerts:   d.alertEngine,
		GRPCMon:  d.grpcMon,
		Exporter: d.getExporter(),
		HotReload: func() hotreload.State {
			return d.HotReloadState()
		},
	}

	d.dashboard = dashboard.NewServer(addr, providers)

	// Wire real-time log push to dashboard WebSocket
	d.Log.SetOnEntry(func(entry log.LogEntry) {
		if d.dashboard != nil {
			d.dashboard.Hub().Broadcast(dashboard.Event{
				Type: "log",
				Data: entry,
			})
		}
	})

	// Wire real-time request push — update callback on the existing inspector
	// so the handler reference stays valid
	d.inspector.SetOnLog(func(rl middleware.RequestLog) {
		d.Log.Debug("http request",
			"method", rl.Method,
			"path", rl.Path,
			"status", rl.StatusCode,
			"duration", rl.Duration,
		)
		if d.dashboard != nil {
			d.dashboard.Hub().Broadcast(dashboard.Event{
				Type: "request",
				Data: rl,
			})
		}
	})

	if err := d.dashboard.Start(); err != nil {
		return err
	}

	d.Log.Info("dashboard started", "addr", addr, "url", "http://localhost"+addr)
	return nil
}

// StopDashboard stops the web dashboard server.
func (d *DevTool) StopDashboard() error {
	if d.dashboard != nil {
		return d.dashboard.Stop()
	}
	return nil
}

// Shutdown stops all background monitors and the dashboard gracefully.
func (d *DevTool) Shutdown() {
	d.StopGoroutineMonitor()
	d.StopMemStats()
	d.StopAlerts()
	d.StopHotReload()
	d.StopDashboard()
}

// --- Phase 4: DB Logger, Timeline, Config Viewer ---

// DBLogger returns the database query logger.
// Use dblog.WrapDB(db, dt.DBLogger()) to wrap a *sql.DB.
func (d *DevTool) DBLogger() *dblog.Logger {
	return d.dbLogger
}

// Timeline returns the event timeline.
func (d *DevTool) Timeline() *timeline.Timeline {
	return d.timeline
}

// TimelineRecord adds a point-in-time event to the timeline.
func (d *DevTool) TimelineRecord(category, label string, data map[string]any) string {
	if !d.enabled {
		return ""
	}
	return d.timeline.Record(category, label, data)
}

// TimelineStart begins a span on the timeline. Call span.End() to complete it.
func (d *DevTool) TimelineStart(category, label string, data map[string]any) *timeline.Span {
	return d.timeline.Start(category, label, data)
}

// PrintTimeline prints recent timeline events to the configured output.
func (d *DevTool) PrintTimeline(n int) {
	if !d.enabled {
		return
	}
	events := d.timeline.LastEvents(n)
	fmt.Fprint(d.output, timeline.FormatEvents(events, d.isColorized()))
}

// Config returns the configuration viewer.
func (d *DevTool) Config() *config.Viewer {
	return d.configView
}

// RegisterConfig adds a named configuration struct for display on the dashboard.
// Fields tagged `devtool:"redact"` will have their values masked.
func (d *DevTool) RegisterConfig(name string, cfg any, sources ...map[string]string) {
	d.configView.Register(name, cfg, sources...)
}

// PrintConfig prints all registered configurations to the configured output.
func (d *DevTool) PrintConfig() {
	if !d.enabled {
		return
	}
	for _, snap := range d.configView.Snapshot() {
		fmt.Fprint(d.output, config.FormatSnapshot(snap, d.isColorized()))
	}
}

// --- Phase 5: Environment, Dependencies, Error Tracking, Profiler ---

// Environ returns the detected environment info.
func (d *DevTool) Environ() environ.Info {
	return d.envInfo
}

// PrintEnviron prints environment info to the configured output.
func (d *DevTool) PrintEnviron() {
	if !d.enabled {
		return
	}
	fmt.Fprint(d.output, environ.FormatInfo(d.envInfo, d.isColorized()))
}

// Dependencies returns the module dependency scan result.
func (d *DevTool) Dependencies() deps.ScanResult {
	if d.depsResult == nil {
		result, _ := deps.ScanFromBuildInfo()
		d.depsResult = &result
	}
	return *d.depsResult
}

// PrintDependencies prints dependencies to the configured output.
func (d *DevTool) PrintDependencies() {
	if !d.enabled {
		return
	}
	fmt.Fprint(d.output, deps.FormatScanResult(d.Dependencies(), d.isColorized()))
}

// ErrorTracker returns the error tracker.
func (d *DevTool) ErrorTracker() *errtrack.Tracker {
	return d.errTracker
}

// TrackError records an error occurrence.
func (d *DevTool) TrackError(err error, data ...map[string]any) {
	if !d.enabled {
		return
	}
	d.errTracker.Track(err, data...)
}

// RecoverMiddleware returns an HTTP handler that recovers panics and tracks them.
func (d *DevTool) RecoverMiddleware(next http.Handler) http.Handler {
	return d.errTracker.RecoverMiddleware(next)
}

// PrintErrorStats prints error statistics to the configured output.
func (d *DevTool) PrintErrorStats() {
	if !d.enabled {
		return
	}
	fmt.Fprint(d.output, errtrack.FormatStats(d.errTracker.Stats(), d.isColorized()))
}

// Profiler returns the profiler.
func (d *DevTool) Profiler() *profiler.Profiler {
	return d.prof
}

// CaptureCPUProfile captures a CPU profile for the given duration.
func (d *DevTool) CaptureCPUProfile(duration time.Duration) (*profiler.Profile, error) {
	return d.prof.CaptureCPU(duration)
}

// CaptureHeapProfile captures a heap profile snapshot.
func (d *DevTool) CaptureHeapProfile() (*profiler.Profile, error) {
	return d.prof.CaptureHeap()
}

// --- Phase 6: HTTP Client Tracer, Cache Monitor, Rate Limiter, Benchmarks ---

// WrapHTTPClient returns an instrumented *http.Client that traces outgoing requests.
func (d *DevTool) WrapHTTPClient(client *http.Client) *http.Client {
	return d.httpTracer.WrapClient(client)
}

// HTTPTracer returns the HTTP client tracer.
func (d *DevTool) HTTPTracer() *httptrace.Tracer {
	return d.httpTracer
}

// CacheMonitor returns the cache monitor.
func (d *DevTool) CacheMonitor() *cachemon.Monitor {
	return d.cacheMon
}

// RegisterCache creates a named cache tracker.
func (d *DevTool) RegisterCache(name string) *cachemon.Recorder {
	return d.cacheMon.Register(name)
}

// RateLimitMonitor returns the rate limiter monitor.
func (d *DevTool) RateLimitMonitor() *ratelimit.Monitor {
	return d.rateMon
}

// RegisterRateLimiter creates a named rate limiter tracker.
func (d *DevTool) RegisterRateLimiter(name string) *ratelimit.Recorder {
	return d.rateMon.Register(name)
}

// Benchmark runs fn n times and returns statistics.
func (d *DevTool) Benchmark(label string, n int, fn func()) bench.Result {
	return d.benchRunner.Run(label, n, fn)
}

// BenchRunner returns the benchmark runner.
func (d *DevTool) BenchRunner() *bench.Runner {
	return d.benchRunner
}

// --- Phase 7: Alerts, Export, gRPC, Hot Reload ---

// AlertEngine returns the alert rules engine.
func (d *DevTool) AlertEngine() *alerts.Engine {
	return d.alertEngine
}

// AddAlertRule registers an alert rule.
func (d *DevTool) AddAlertRule(rule alerts.Rule) {
	d.alertEngine.AddRule(rule)
}

// AlertOnGoroutineCount adds a rule that fires when goroutine count exceeds threshold.
func (d *DevTool) AlertOnGoroutineCount(threshold int) {
	d.alertEngine.AddRule(alerts.GoroutineCountRule(threshold, alerts.SeverityWarning))
}

// AlertOnHeapAlloc adds a rule that fires when heap allocation exceeds threshold bytes.
func (d *DevTool) AlertOnHeapAlloc(thresholdBytes uint64) {
	d.alertEngine.AddRule(alerts.HeapAllocRule(thresholdBytes, alerts.SeverityWarning))
}

// StartAlerts begins periodic alert rule evaluation.
func (d *DevTool) StartAlerts(interval time.Duration) {
	if !d.enabled {
		return
	}
	d.alertEngine = alerts.New(
		alerts.WithCheckInterval(interval),
		alerts.WithOnAlert(func(a alerts.Alert) {
			if a.State == alerts.StateFiring {
				d.Log.Warn("alert firing", "rule", a.RuleName, "value", a.Value, "severity", a.Severity)
			} else {
				d.Log.Info("alert resolved", "rule", a.RuleName)
			}
			if d.dashboard != nil {
				d.dashboard.Hub().Broadcast(dashboard.Event{
					Type: "alert",
					Data: a,
				})
			}
		}),
	)
	d.alertEngine.Start()
	d.Log.Debug("alert engine started", "interval", interval)
}

// StopAlerts stops the alert engine.
func (d *DevTool) StopAlerts() {
	if d.alertEngine != nil {
		d.alertEngine.Stop()
	}
}

// ExportJSON captures a full debug snapshot as JSON.
func (d *DevTool) ExportJSON() ([]byte, error) {
	return d.getExporter().CaptureJSON()
}

// ExportHTML captures a full debug snapshot as a self-contained HTML report.
func (d *DevTool) ExportHTML() ([]byte, error) {
	return d.getExporter().CaptureHTML()
}

// ExportToFile writes a debug snapshot to a file.
func (d *DevTool) ExportToFile(path, format string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return d.getExporter().WriteTo(f, format)
}

func (d *DevTool) getExporter() *export.Exporter {
	return export.New(export.DataSource{
		AppName:    d.opts.appName,
		Environment: func() any { return d.envInfo },
		Logs:       func() any { return d.Log.History() },
		Requests:   func() any { return d.inspector.Requests() },
		Goroutines: func() any { return d.GoroutineSnapshot() },
		MemStats:   func() any { return d.MemSnapshot() },
		Timers:     func() any { return d.report.All() },
		Queries:    func() any { return d.dbLogger.Queries() },
		Timeline:   func() any { return d.timeline.Events() },
		Config:     func() any { return d.configView.Snapshot() },
		Errors:     func() any { return d.errTracker.Stats() },
		Outgoing:   func() any { return d.httpTracer.Traces() },
		Caches:     func() any { return d.cacheMon.Stats() },
		RateLimits: func() any { return d.rateMon.Stats() },
		Benchmarks: func() any { return d.benchRunner.Results() },
		Alerts:     func() any { return d.alertEngine.Alerts() },
	})
}

// GRPCMonitor returns the gRPC call monitor.
func (d *DevTool) GRPCMonitor() *grpcmon.Monitor {
	return d.grpcMon
}

// StartHotReload begins file watching and auto-rebuild.
func (d *DevTool) StartHotReload(opts ...hotreload.Option) error {
	d.hotReload = hotreload.New(opts...)
	if err := d.hotReload.Start(); err != nil {
		return err
	}
	d.Log.Info("hot reload started")
	return nil
}

// StopHotReload stops the file watcher.
func (d *DevTool) StopHotReload() {
	if d.hotReload != nil {
		d.hotReload.Stop()
	}
}

// HotReloadState returns the current hot reload state.
func (d *DevTool) HotReloadState() hotreload.State {
	if d.hotReload == nil {
		return hotreload.State{Status: hotreload.StatusIdle}
	}
	return d.hotReload.GetState()
}

func (d *DevTool) isColorized() bool {
	if d.opts.colorize != nil {
		return *d.opts.colorize
	}
	if f, ok := d.output.(*os.File); ok {
		return color.IsTerminal(f)
	}
	return false
}
