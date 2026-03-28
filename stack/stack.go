package stack

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tarunnahak/godevtool/internal/color"
)

// Frame represents a single stack frame.
type Frame struct {
	Function  string // fully qualified function name
	ShortFunc string // just the function name
	Package   string // package path
	File      string // absolute file path
	ShortFile string // filename only
	Line      int
}

// Trace represents an ordered list of stack frames.
type Trace struct {
	Frames []Frame
}

// Config controls formatting behavior.
type Config struct {
	Colorize      bool
	FilterRuntime bool   // hide runtime.* frames, default true
	MaxFrames     int    // 0 = unlimited
	RelativeTo    string // base path to make file paths relative
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Colorize:      true,
		FilterRuntime: true,
		MaxFrames:     0,
	}
}

// Capture returns the current stack trace. skip=0 means the caller of Capture.
func Capture(skip int) Trace {
	var pcs [64]uintptr
	n := runtime.Callers(skip+2, pcs[:]) // +2 to skip Callers and Capture
	frames := runtime.CallersFrames(pcs[:n])

	var trace Trace
	for {
		frame, more := frames.Next()
		f := Frame{
			Function:  frame.Function,
			File:      frame.File,
			ShortFile: filepath.Base(frame.File),
			Line:      frame.Line,
		}

		// split function into package and short name
		if idx := strings.LastIndex(frame.Function, "/"); idx >= 0 {
			rest := frame.Function[idx+1:]
			if dotIdx := strings.Index(rest, "."); dotIdx >= 0 {
				f.Package = frame.Function[:idx+1+dotIdx]
				f.ShortFunc = rest[dotIdx+1:]
			} else {
				f.ShortFunc = rest
				f.Package = frame.Function[:idx]
			}
		} else if dotIdx := strings.Index(frame.Function, "."); dotIdx >= 0 {
			f.Package = frame.Function[:dotIdx]
			f.ShortFunc = frame.Function[dotIdx+1:]
		} else {
			f.ShortFunc = frame.Function
		}

		trace.Frames = append(trace.Frames, f)
		if !more {
			break
		}
	}
	return trace
}

// Caller returns a single Frame for the immediate caller.
// skip=0 means the caller of Caller.
func Caller(skip int) Frame {
	t := Capture(skip + 1)
	if len(t.Frames) > 0 {
		return t.Frames[0]
	}
	return Frame{}
}

// Format renders the trace as a readable string.
func (t Trace) Format(cfg Config) string {
	var b strings.Builder
	frames := t.Frames

	if cfg.FilterRuntime {
		frames = filterRuntime(frames)
	}
	if cfg.MaxFrames > 0 && len(frames) > cfg.MaxFrames {
		frames = frames[:cfg.MaxFrames]
	}

	// find max function width for alignment
	maxFunc := 0
	for _, f := range frames {
		name := f.ShortFunc
		if f.Package != "" {
			name = f.Package + "." + f.ShortFunc
		}
		if len(name) > maxFunc {
			maxFunc = len(name)
		}
	}

	for _, f := range frames {
		funcName := f.ShortFunc
		if f.Package != "" {
			funcName = f.Package + "." + f.ShortFunc
		}

		file := f.ShortFile
		if cfg.RelativeTo != "" {
			if rel, err := filepath.Rel(cfg.RelativeTo, f.File); err == nil {
				file = rel
			}
		}

		loc := fmt.Sprintf("%s:%d", file, f.Line)

		b.WriteString("  ")
		b.WriteString(color.Wrap(fmt.Sprintf("%-*s", maxFunc, funcName), cfg.Colorize, color.Cyan))
		b.WriteString("  ")
		b.WriteString(color.Wrap(loc, cfg.Colorize, color.Gray))
		b.WriteByte('\n')
	}
	return b.String()
}

func filterRuntime(frames []Frame) []Frame {
	var filtered []Frame
	for _, f := range frames {
		if strings.HasPrefix(f.Package, "runtime") {
			continue
		}
		if strings.HasPrefix(f.Function, "runtime.") {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}
