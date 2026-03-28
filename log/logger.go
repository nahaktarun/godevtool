package log

import (
	"io"
	"os"
	"sync"
	"time"
)

// Logger provides structured, colorized logging.
// It is safe for concurrent use.
type Logger struct {
	out        io.Writer
	level      Level
	colorize   bool
	timeFormat string
	prefix     string
	fields     []field
	mu         sync.Mutex
	enabled    bool
}

// New creates a Logger.
func New(out io.Writer, level Level, colorize bool, timeFormat string) *Logger {
	if out == nil {
		out = os.Stdout
	}
	if timeFormat == "" {
		timeFormat = "15:04:05.000"
	}
	return &Logger{
		out:        out,
		level:      level,
		colorize:   colorize,
		timeFormat: timeFormat,
		enabled:    true,
	}
}

// With returns a child Logger that includes the given key-value fields
// in every subsequent log line.
func (l *Logger) With(keyvals ...any) *Logger {
	child := &Logger{
		out:        l.out,
		level:      l.level,
		colorize:   l.colorize,
		timeFormat: l.timeFormat,
		prefix:     l.prefix,
		enabled:    l.enabled,
		fields:     make([]field, len(l.fields), len(l.fields)+len(keyvals)/2),
	}
	copy(child.fields, l.fields)
	child.fields = append(child.fields, parseFields(keyvals)...)
	return child
}

// WithPrefix returns a child Logger with the given prefix.
func (l *Logger) WithPrefix(prefix string) *Logger {
	child := l.With()
	child.prefix = prefix
	return child
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	l.log(LevelDebug, msg, args)
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	l.log(LevelInfo, msg, args)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) {
	l.log(LevelWarn, msg, args)
}

// Error logs at error level. If the first arg is an error, it is
// automatically added as "error" key.
func (l *Logger) Error(msg string, args ...any) {
	if len(args) > 0 {
		if err, ok := args[0].(error); ok {
			args = append([]any{"error", err}, args[1:]...)
		}
	}
	l.log(LevelError, msg, args)
}

// SetLevel changes the minimum log level at runtime.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetEnabled enables or disables the logger.
func (l *Logger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

func (l *Logger) log(level Level, msg string, args []any) {
	l.mu.Lock()
	if !l.enabled || level < l.level {
		l.mu.Unlock()
		return
	}
	out := l.out
	colorize := l.colorize
	timeFmt := l.timeFormat
	prefix := l.prefix
	// merge persistent fields with call-site fields
	fields := make([]field, len(l.fields), len(l.fields)+len(args)/2)
	copy(fields, l.fields)
	l.mu.Unlock()

	fields = append(fields, parseFields(args)...)

	e := entry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Fields:  fields,
		Prefix:  prefix,
	}
	formatEntry(out, e, colorize, timeFmt)
}

func parseFields(args []any) []field {
	var fields []field
	for i := 0; i < len(args)-1; i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, field{Key: key, Value: args[i+1]})
	}
	return fields
}
