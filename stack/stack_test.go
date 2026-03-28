package stack

import (
	"strings"
	"testing"
)

func TestCapture(t *testing.T) {
	trace := Capture(0)

	if len(trace.Frames) == 0 {
		t.Fatal("expected at least one frame")
	}

	// first frame should be this test function
	f := trace.Frames[0]
	if !strings.Contains(f.Function, "TestCapture") {
		t.Errorf("first frame function = %q, expected TestCapture", f.Function)
	}
	if f.Line == 0 {
		t.Error("expected non-zero line number")
	}
}

func TestCaller(t *testing.T) {
	f := Caller(0)

	if !strings.Contains(f.Function, "TestCaller") {
		t.Errorf("Caller function = %q, expected TestCaller", f.Function)
	}
	if f.ShortFile != "stack_test.go" {
		t.Errorf("ShortFile = %q, expected stack_test.go", f.ShortFile)
	}
}

func TestFormat(t *testing.T) {
	trace := Capture(0)
	cfg := Config{
		Colorize:      false,
		FilterRuntime: true,
	}
	formatted := trace.Format(cfg)

	if !strings.Contains(formatted, "TestFormat") {
		t.Errorf("expected TestFormat in formatted output:\n%s", formatted)
	}
	if !strings.Contains(formatted, "stack_test.go") {
		t.Errorf("expected stack_test.go in formatted output:\n%s", formatted)
	}
}

func TestFilterRuntime(t *testing.T) {
	trace := Capture(0)

	cfg := Config{
		Colorize:      false,
		FilterRuntime: false,
	}
	unfiltered := trace.Format(cfg)

	cfg.FilterRuntime = true
	filtered := trace.Format(cfg)

	// filtered should be shorter or equal
	if len(filtered) > len(unfiltered) {
		t.Error("filtered should not be longer than unfiltered")
	}
}

func TestMaxFrames(t *testing.T) {
	trace := Capture(0)
	cfg := Config{
		Colorize:      false,
		FilterRuntime: false,
		MaxFrames:     1,
	}
	formatted := trace.Format(cfg)
	lines := strings.Split(strings.TrimSpace(formatted), "\n")

	if len(lines) > 1 {
		t.Errorf("MaxFrames=1 but got %d lines", len(lines))
	}
}
