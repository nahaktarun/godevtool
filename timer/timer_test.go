package timer

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTimerBasic(t *testing.T) {
	tm := Start("test-op", nil)
	time.Sleep(10 * time.Millisecond)
	elapsed := tm.Stop()

	if elapsed < 10*time.Millisecond {
		t.Errorf("elapsed %v < 10ms", elapsed)
	}
}

func TestTimerDoubleStop(t *testing.T) {
	tm := Start("test-op", nil)
	time.Sleep(5 * time.Millisecond)
	d1 := tm.Stop()
	time.Sleep(10 * time.Millisecond)
	d2 := tm.Stop()

	if d1 != d2 {
		t.Errorf("double stop should return same duration: %v != %v", d1, d2)
	}
}

func TestTimerCallback(t *testing.T) {
	var gotLabel string
	var gotDuration time.Duration

	tm := Start("callback-test", func(label string, d time.Duration) {
		gotLabel = label
		gotDuration = d
	})
	time.Sleep(5 * time.Millisecond)
	tm.Stop()

	if gotLabel != "callback-test" {
		t.Errorf("label = %q, want callback-test", gotLabel)
	}
	if gotDuration < 5*time.Millisecond {
		t.Errorf("duration %v < 5ms", gotDuration)
	}
}

func TestTimerString(t *testing.T) {
	tm := Start("my-op", nil)
	time.Sleep(5 * time.Millisecond)
	tm.Stop()
	s := tm.String()

	if !strings.HasPrefix(s, "my-op: ") {
		t.Errorf("String() = %q, expected prefix 'my-op: '", s)
	}
}

func TestReportRecord(t *testing.T) {
	r := NewReport()
	r.Record("op1", 100*time.Millisecond)
	r.Record("op1", 200*time.Millisecond)
	r.Record("op2", 50*time.Millisecond)

	s, ok := r.Get("op1")
	if !ok {
		t.Fatal("op1 not found")
	}
	if s.Count != 2 {
		t.Errorf("count = %d, want 2", s.Count)
	}
	if s.Min != 100*time.Millisecond {
		t.Errorf("min = %v, want 100ms", s.Min)
	}
	if s.Max != 200*time.Millisecond {
		t.Errorf("max = %v, want 200ms", s.Max)
	}
}

func TestReportAll(t *testing.T) {
	r := NewReport()
	r.Record("fast", 10*time.Millisecond)
	r.Record("slow", 100*time.Millisecond)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2", len(all))
	}
	// sorted by total desc
	if all[0].Label != "slow" {
		t.Errorf("first should be slow, got %s", all[0].Label)
	}
}

func TestReportPrintTo(t *testing.T) {
	r := NewReport()
	r.Record("op1", 100*time.Millisecond)

	var buf bytes.Buffer
	r.PrintTo(&buf)
	output := buf.String()

	if !strings.Contains(output, "op1") {
		t.Errorf("expected op1 in output: %s", output)
	}
	if !strings.Contains(output, "LABEL") {
		t.Errorf("expected header: %s", output)
	}
}

func TestReportReset(t *testing.T) {
	r := NewReport()
	r.Record("op1", 100*time.Millisecond)
	r.Reset()

	if _, ok := r.Get("op1"); ok {
		t.Error("op1 should not exist after reset")
	}
}
