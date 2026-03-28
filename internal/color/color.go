package color

import (
	"fmt"
	"os"
	"strings"
)

// Code represents an ANSI color/style code.
type Code int

const (
	Reset  Code = 0
	Bold   Code = 1
	Dim    Code = 2
	Red    Code = 31
	Green  Code = 32
	Yellow Code = 33
	Blue   Code = 34
	Cyan   Code = 36
	Gray   Code = 90
	White  Code = 97
)

// Wrap returns s wrapped in ANSI escape codes. If enabled is false, returns s unchanged.
func Wrap(s string, enabled bool, codes ...Code) string {
	if !enabled || len(codes) == 0 {
		return s
	}
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = fmt.Sprintf("%d", int(c))
	}
	return fmt.Sprintf("\033[%sm%s\033[0m", strings.Join(parts, ";"), s)
}

// Sprintf formats and wraps in color.
func Sprintf(enabled bool, codes []Code, format string, args ...any) string {
	return Wrap(fmt.Sprintf(format, args...), enabled, codes...)
}

// IsTerminal returns true if the file descriptor appears to be a terminal.
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
