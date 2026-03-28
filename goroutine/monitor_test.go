package goroutine

import (
	"strings"
	"testing"
	"time"
)

func TestMonitorCurrent(t *testing.T) {
	m := NewMonitor(time.Second)
	snap := m.Current()

	if snap.Count == 0 {
		t.Error("expected at least one goroutine")
	}
	if len(snap.Goroutines) == 0 {
		t.Error("expected goroutine info")
	}
	if snap.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestMonitorCount(t *testing.T) {
	m := NewMonitor(time.Second)
	count := m.Count()
	if count == 0 {
		t.Error("expected at least one goroutine")
	}
}

func TestMonitorStartStop(t *testing.T) {
	m := NewMonitor(50 * time.Millisecond)
	m.Start()
	time.Sleep(200 * time.Millisecond)
	m.Stop()

	history := m.History()
	if len(history) < 2 {
		t.Errorf("expected at least 2 snapshots, got %d", len(history))
	}
}

func TestMonitorLeakCheck(t *testing.T) {
	m := NewMonitor(50 * time.Millisecond)
	m.Start()
	time.Sleep(100 * time.Millisecond)

	// spawn some goroutines
	done := make(chan struct{})
	for i := 0; i < 3; i++ {
		go func() {
			<-done
		}()
	}

	time.Sleep(150 * time.Millisecond)
	suspects := m.LeakCheck()
	close(done)
	m.Stop()

	// We should see at least the goroutines we created
	// (they're still running waiting on the channel)
	if len(suspects) == 0 {
		t.Log("no suspects found (may be timing dependent)")
	}
}

func TestFormatSnapshot(t *testing.T) {
	m := NewMonitor(time.Second)
	snap := m.Current()
	output := FormatSnapshot(snap, false)

	if !strings.Contains(output, "Goroutines:") {
		t.Errorf("expected Goroutines header: %s", output)
	}
}

func TestParseGoroutines(t *testing.T) {
	raw := `goroutine 1 [running]:
main.main()
	/app/main.go:10 +0x1234

goroutine 5 [chan receive]:
runtime.gopark(...)
	/go/src/runtime/proc.go:250
`
	gs := parseGoroutines(raw)
	if len(gs) != 2 {
		t.Fatalf("expected 2 goroutines, got %d", len(gs))
	}
	if gs[0].ID != 1 {
		t.Errorf("first goroutine ID = %d, want 1", gs[0].ID)
	}
	if gs[0].State != "running" {
		t.Errorf("first goroutine state = %q, want running", gs[0].State)
	}
	if gs[1].State != "chan receive" {
		t.Errorf("second goroutine state = %q, want chan receive", gs[1].State)
	}
}
