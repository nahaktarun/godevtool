package timer

import (
	"fmt"
	"sync"
	"time"
)

// Timer measures elapsed time for a labeled operation.
type Timer struct {
	Label     string
	StartTime time.Time
	EndTime   time.Time
	stopped   bool
	onStop    func(label string, elapsed time.Duration)
	mu        sync.Mutex
}

// Start creates and starts a new Timer.
//
//	defer timer.Start("operation", nil).Stop()
func Start(label string, onStop func(string, time.Duration)) *Timer {
	return &Timer{
		Label:     label,
		StartTime: time.Now(),
		onStop:    onStop,
	}
}

// Stop records the end time and invokes the onStop callback.
// Safe to call multiple times; subsequent calls are no-ops.
// Returns the elapsed duration.
func (t *Timer) Stop() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return t.EndTime.Sub(t.StartTime)
	}
	t.EndTime = time.Now()
	t.stopped = true
	elapsed := t.EndTime.Sub(t.StartTime)

	if t.onStop != nil {
		t.onStop(t.Label, elapsed)
	}
	return elapsed
}

// Elapsed returns duration since start. If stopped, returns final duration.
func (t *Timer) Elapsed() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return t.EndTime.Sub(t.StartTime)
	}
	return time.Since(t.StartTime)
}

// String returns "label: 123.456ms".
func (t *Timer) String() string {
	return fmt.Sprintf("%s: %s", t.Label, t.Elapsed())
}
