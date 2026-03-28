package memstats

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCollectorCurrent(t *testing.T) {
	c := NewCollector(time.Second, 10)
	snap := c.Current()

	if snap.HeapAlloc == 0 {
		t.Error("expected non-zero HeapAlloc")
	}
	if snap.Sys == 0 {
		t.Error("expected non-zero Sys")
	}
	if snap.Goroutines == 0 {
		t.Error("expected at least one goroutine")
	}
	if snap.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestCollectorStartStop(t *testing.T) {
	c := NewCollector(50*time.Millisecond, 100)
	c.Start()
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	history := c.History()
	if len(history) < 2 {
		t.Errorf("expected at least 2 snapshots, got %d", len(history))
	}
}

func TestSnapshotFormatted(t *testing.T) {
	c := NewCollector(time.Second, 10)
	snap := c.Current()

	s := snap.HeapAllocStr()
	if s == "" {
		t.Error("HeapAllocStr empty")
	}

	s = snap.SysStr()
	if s == "" {
		t.Error("SysStr empty")
	}
}

func TestFormatSnapshot(t *testing.T) {
	c := NewCollector(time.Second, 10)
	snap := c.Current()
	output := FormatSnapshot(snap, false)

	checks := []string{"Memory Stats", "Heap Alloc", "GC Cycles", "Goroutines"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected %q in output:\n%s", check, output)
		}
	}
}

func TestPrintSnapshot(t *testing.T) {
	c := NewCollector(time.Second, 10)
	snap := c.Current()

	var buf bytes.Buffer
	PrintSnapshot(&buf, snap, false)

	if buf.Len() == 0 {
		t.Error("PrintSnapshot produced no output")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}
	for _, tc := range tests {
		got := formatBytes(tc.input)
		if got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
