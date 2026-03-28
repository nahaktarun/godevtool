package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelInfo, false, "15:04:05")

	l.Debug("should not appear")
	l.Info("should appear")
	l.Warn("should appear too")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("Debug message should not appear at Info level")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("Info message should appear at Info level")
	}
	if !strings.Contains(output, "should appear too") {
		t.Error("Warn message should appear at Info level")
	}
}

func TestLoggerKeyValues(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, false, "15:04:05")

	l.Info("test message", "key1", "value1", "key2", 42)

	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Errorf("expected key1=value1 in output: %s", output)
	}
	if !strings.Contains(output, "key2=42") {
		t.Errorf("expected key2=42 in output: %s", output)
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, false, "15:04:05")

	child := l.With("request_id", "abc-123")
	child.Info("child log")

	output := buf.String()
	if !strings.Contains(output, "request_id=abc-123") {
		t.Errorf("expected request_id in child log: %s", output)
	}
}

func TestLoggerWithPrefix(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, false, "15:04:05")

	child := l.WithPrefix("myapp")
	child.Info("prefixed")

	output := buf.String()
	if !strings.Contains(output, "[myapp]") {
		t.Errorf("expected [myapp] prefix: %s", output)
	}
}

func TestLoggerSetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelInfo, false, "15:04:05")

	l.Debug("before")
	l.SetLevel(LevelDebug)
	l.Debug("after")

	output := buf.String()
	if strings.Contains(output, "before") {
		t.Error("Debug before SetLevel should not appear")
	}
	if !strings.Contains(output, "after") {
		t.Error("Debug after SetLevel should appear")
	}
}

func TestLoggerDisable(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, false, "15:04:05")

	l.SetEnabled(false)
	l.Info("should not appear")
	l.SetEnabled(true)
	l.Info("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("Disabled logger should produce no output")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("Re-enabled logger should produce output")
	}
}

func TestLoggerErrorWithError(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, false, "15:04:05")

	l.Error("something failed", strings.NewReader("not-an-error"), "extra", "data")
	output1 := buf.String()
	buf.Reset()

	// When first arg IS an error, it should be added as error= key
	l.Error("something failed", &testError{"conn refused"}, "host", "db.local")
	output2 := buf.String()

	if !strings.Contains(output2, "error=") {
		t.Errorf("expected error= key when first arg is error: %s", output2)
	}
	_ = output1
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
