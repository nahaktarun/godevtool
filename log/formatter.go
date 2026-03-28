package log

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nahaktarun/godevtool/internal/color"
)

type entry struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  []field
	Prefix  string
}

type field struct {
	Key   string
	Value any
}

func levelColor(l Level) color.Code {
	switch l {
	case LevelDebug:
		return color.Cyan
	case LevelInfo:
		return color.Green
	case LevelWarn:
		return color.Yellow
	case LevelError:
		return color.Red
	default:
		return color.White
	}
}

func formatEntry(w io.Writer, e entry, colorize bool, timeFmt string) {
	var b strings.Builder

	// timestamp
	ts := e.Time.Format(timeFmt)
	b.WriteString(color.Wrap(ts, colorize, color.Gray))
	b.WriteByte(' ')

	// level (padded to 5 chars)
	lvl := fmt.Sprintf("%-5s", e.Level.String())
	b.WriteString(color.Wrap(lvl, colorize, levelColor(e.Level), color.Bold))
	b.WriteByte(' ')

	// prefix
	if e.Prefix != "" {
		b.WriteString(color.Wrap("["+e.Prefix+"]", colorize, color.Cyan))
		b.WriteByte(' ')
	}

	// message (padded for alignment)
	msg := e.Message
	if len(msg) < 40 {
		msg = msg + strings.Repeat(" ", 40-len(msg))
	}
	b.WriteString(msg)

	// key=value pairs
	for _, f := range e.Fields {
		b.WriteByte(' ')
		b.WriteString(color.Wrap(f.Key, colorize, color.Blue))
		b.WriteByte('=')
		val := formatValue(f.Value)
		b.WriteString(color.Wrap(val, colorize, color.White))
	}

	b.WriteByte('\n')
	fmt.Fprint(w, b.String())
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		if strings.ContainsAny(val, " \t\n\"") {
			return fmt.Sprintf("%q", val)
		}
		return val
	case error:
		s := val.Error()
		if strings.ContainsAny(s, " \t\n\"") {
			return fmt.Sprintf("%q", s)
		}
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}
