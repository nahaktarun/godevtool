package godevtool

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nahaktarun/godevtool/log"
)

func TestNew(t *testing.T) {
	dt := New()
	if dt == nil {
		t.Fatal("New() returned nil")
	}
	if dt.Log == nil {
		t.Fatal("Log is nil")
	}
}

func TestNewWithOptions(t *testing.T) {
	var buf bytes.Buffer
	dt := New(
		WithOutput(&buf),
		WithLogLevel(log.LevelDebug),
		WithNoColor(),
		WithAppName("test"),
	)

	dt.Log.Info("hello", "key", "value")
	output := buf.String()

	if !strings.Contains(output, "[test]") {
		t.Errorf("expected [test] prefix: %s", output)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("expected hello: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected key=value: %s", output)
	}
}

func TestInspect(t *testing.T) {
	var buf bytes.Buffer
	dt := New(WithOutput(&buf), WithNoColor())

	type Sample struct {
		Name string
		Age  int
	}
	s := Sample{Name: "Bob", Age: 25}
	result := dt.Inspect(s)

	if !strings.Contains(result, "Bob") {
		t.Errorf("expected Bob in inspect: %s", result)
	}
	if !strings.Contains(result, "25") {
		t.Errorf("expected 25 in inspect: %s", result)
	}
}

func TestTimer(t *testing.T) {
	var buf bytes.Buffer
	dt := New(WithOutput(&buf), WithNoColor(), WithLogLevel(log.LevelDebug))

	tm := dt.Timer("test-op")
	tm.Stop()

	// should have recorded to the report
	stats, ok := dt.TimerReport().Get("test-op")
	if !ok {
		t.Fatal("timer not recorded in report")
	}
	if stats.Count != 1 {
		t.Errorf("count = %d, want 1", stats.Count)
	}
}

func TestStack(t *testing.T) {
	var buf bytes.Buffer
	dt := New(WithOutput(&buf), WithNoColor())

	s := dt.Stack(0)
	if !strings.Contains(s, "TestStack") {
		t.Errorf("expected TestStack in trace: %s", s)
	}
}

func TestDisableEnable(t *testing.T) {
	var buf bytes.Buffer
	dt := New(WithOutput(&buf), WithNoColor())

	dt.Disable()
	dt.Log.Info("should not appear")
	result := dt.Inspect("hidden")

	if buf.Len() > 0 {
		t.Errorf("disabled devtool should produce no output: %s", buf.String())
	}
	if result != "" {
		t.Errorf("disabled inspect should return empty: %s", result)
	}

	dt.Enable()
	dt.Log.Info("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Error("re-enabled devtool should produce output")
	}
}
