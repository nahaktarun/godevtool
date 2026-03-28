package godevtool

import (
	"io"

	"github.com/tarunnahak/godevtool/log"
)

// Option configures a DevTool instance.
type Option func(*options)

type options struct {
	output     io.Writer
	logLevel   log.Level
	colorize   *bool // nil = auto-detect
	timeFormat string
	appName    string
	maxDepth   int
}

func defaultOptions() options {
	return options{
		logLevel:   log.LevelInfo,
		timeFormat: "15:04:05.000",
		maxDepth:   10,
	}
}

// WithOutput sets the writer for all devtool output.
func WithOutput(w io.Writer) Option {
	return func(o *options) { o.output = w }
}

// WithLogLevel sets the minimum log level.
func WithLogLevel(level log.Level) Option {
	return func(o *options) { o.logLevel = level }
}

// WithNoColor disables ANSI color codes.
func WithNoColor() Option {
	return func(o *options) {
		f := false
		o.colorize = &f
	}
}

// WithColor forces ANSI color codes on.
func WithColor() Option {
	return func(o *options) {
		t := true
		o.colorize = &t
	}
}

// WithTimeFormat sets the timestamp format (Go reference time layout).
func WithTimeFormat(format string) Option {
	return func(o *options) { o.timeFormat = format }
}

// WithAppName sets an application name prefix for log lines.
func WithAppName(name string) Option {
	return func(o *options) { o.appName = name }
}

// WithMaxDepth sets the maximum recursion depth for Inspect.
func WithMaxDepth(depth int) Option {
	return func(o *options) { o.maxDepth = depth }
}
