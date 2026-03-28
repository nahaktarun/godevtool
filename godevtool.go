package godevtool

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tarunnahak/godevtool/inspect"
	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/log"
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

	return &DevTool{
		Log:     logger,
		opts:    o,
		output:  out,
		enabled: true,
		report:  timer.NewReport(),
	}
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

func (d *DevTool) isColorized() bool {
	if d.opts.colorize != nil {
		return *d.opts.colorize
	}
	if f, ok := d.output.(*os.File); ok {
		return color.IsTerminal(f)
	}
	return false
}
