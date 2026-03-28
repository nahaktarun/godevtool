package godevtool

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tarunnahak/godevtool/dashboard"
	"github.com/tarunnahak/godevtool/goroutine"
	"github.com/tarunnahak/godevtool/inspect"
	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/log"
	"github.com/tarunnahak/godevtool/memstats"
	"github.com/tarunnahak/godevtool/middleware"
	"github.com/tarunnahak/godevtool/stack"
	"github.com/tarunnahak/godevtool/timer"
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
		Timer: d.report,
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

	// Wire real-time request push
	origInspector := d.inspector
	_ = origInspector // already has onLog wired; add dashboard broadcast
	// We re-set the middleware callback to also broadcast to dashboard
	d.inspector = middleware.New(
		middleware.WithOnLog(func(rl middleware.RequestLog) {
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
		}),
	)

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
	d.StopDashboard()
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
