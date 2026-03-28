package bench

import (
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	r := New()

	result := r.Run("test-op", 100, func() {
		time.Sleep(100 * time.Microsecond)
	})

	if result.Label != "test-op" {
		t.Errorf("label = %q", result.Label)
	}
	if result.Iterations != 100 {
		t.Errorf("iterations = %d", result.Iterations)
	}
	if result.TotalTime <= 0 {
		t.Error("total time = 0")
	}
	if result.AvgTime <= 0 {
		t.Error("avg time = 0")
	}
	if result.MinTime <= 0 {
		t.Error("min time = 0")
	}
	if result.MaxTime < result.MinTime {
		t.Error("max < min")
	}
	if result.P50 <= 0 {
		t.Error("P50 = 0")
	}
	if result.P90 <= 0 {
		t.Error("P90 = 0")
	}
	if result.P99 <= 0 {
		t.Error("P99 = 0")
	}
	if result.OpsPerSec <= 0 {
		t.Error("ops/sec = 0")
	}

	// P50 <= P90 <= P99
	if result.P50 > result.P90 {
		t.Errorf("P50 (%v) > P90 (%v)", result.P50, result.P90)
	}
	if result.P90 > result.P99 {
		t.Errorf("P90 (%v) > P99 (%v)", result.P90, result.P99)
	}
}

func TestRunWithSetup(t *testing.T) {
	r := New()
	var setupCount int

	result := r.RunWithSetup("setup-test", 10, func() {
		setupCount++
	}, func() {
		time.Sleep(50 * time.Microsecond)
	})

	if setupCount != 10 {
		t.Errorf("setup called %d times, want 10", setupCount)
	}
	if result.Iterations != 10 {
		t.Errorf("iterations = %d", result.Iterations)
	}
}

func TestRunZeroIterations(t *testing.T) {
	r := New()
	result := r.Run("zero", 0, func() {})
	if result.Iterations != 1 {
		t.Errorf("iterations = %d, want 1 (min)", result.Iterations)
	}
}

func TestResults(t *testing.T) {
	r := New()
	r.Run("op1", 10, func() {})
	r.Run("op2", 10, func() {})

	results := r.Results()
	if len(results) != 2 {
		t.Errorf("results len = %d", len(results))
	}
	if r.Count() != 2 {
		t.Errorf("count = %d", r.Count())
	}
}

func TestLastResults(t *testing.T) {
	r := New()
	for i := 0; i < 5; i++ {
		r.Run("op", 1, func() {})
	}

	last := r.LastResults(2)
	if len(last) != 2 {
		t.Errorf("last len = %d", len(last))
	}
}

func TestCallback(t *testing.T) {
	var called bool
	r := New(WithOnResult(func(res Result) {
		called = true
	}))

	r.Run("test", 1, func() {})
	if !called {
		t.Error("callback not invoked")
	}
}

func TestFormatResult(t *testing.T) {
	r := New()
	result := r.Run("test-op", 100, func() {
		time.Sleep(10 * time.Microsecond)
	})

	output := FormatResult(result, false)
	if !strings.Contains(output, "test-op") {
		t.Errorf("expected label in output: %s", output)
	}
	if !strings.Contains(output, "P99") {
		t.Errorf("expected P99 in output: %s", output)
	}
}

func TestFormatResults(t *testing.T) {
	r := New()
	r.Run("op1", 10, func() {})
	r.Run("op2", 10, func() {})

	output := FormatResults(r.Results(), false)
	if !strings.Contains(output, "op1") {
		t.Errorf("expected op1 in output: %s", output)
	}
}

func TestPercentile(t *testing.T) {
	durations := []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p50 := percentile(durations, 0.50)
	p90 := percentile(durations, 0.90)
	p99 := percentile(durations, 0.99)

	if p50 != 5 {
		t.Errorf("P50 = %v, want 5", p50)
	}
	if p90 < 9 {
		t.Errorf("P90 = %v, expected >= 9", p90)
	}
	if p99 < 9 {
		t.Errorf("P99 = %v, expected >= 9", p99)
	}
}
