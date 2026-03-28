package ratelimit

import (
	"strings"
	"testing"
	"time"
)

func TestAllowedAndThrottled(t *testing.T) {
	m := New()
	r := m.Register("api")

	r.Allowed()
	r.Allowed()
	r.Allowed()
	r.Throttled()

	s, ok := m.StatsFor("api")
	if !ok {
		t.Fatal("api not found")
	}
	if s.Allowed != 3 {
		t.Errorf("allowed = %d", s.Allowed)
	}
	if s.Throttled != 1 {
		t.Errorf("throttled = %d", s.Throttled)
	}
	// 1 throttled / 4 total = 0.25
	if s.ThrottleRate < 0.24 || s.ThrottleRate > 0.26 {
		t.Errorf("throttle rate = %f", s.ThrottleRate)
	}
}

func TestQueued(t *testing.T) {
	m := New()
	r := m.Register("api")

	r.Queued(10 * time.Millisecond)
	r.Queued(20 * time.Millisecond)

	s, _ := m.StatsFor("api")
	if s.Queued != 2 {
		t.Errorf("queued = %d", s.Queued)
	}
	if s.AvgWaitTime != 15*time.Millisecond {
		t.Errorf("avg wait = %v", s.AvgWaitTime)
	}
}

func TestQueueDepth(t *testing.T) {
	m := New()
	r := m.Register("api")

	r.SetQueueDepth(42)
	s, _ := m.StatsFor("api")
	if s.QueueDepth != 42 {
		t.Errorf("queue depth = %d", s.QueueDepth)
	}
}

func TestMultipleLimiters(t *testing.T) {
	m := New()
	m.Register("api")
	m.Register("webhook")

	if m.Count() != 2 {
		t.Errorf("count = %d", m.Count())
	}

	stats := m.Stats()
	if len(stats) != 2 {
		t.Errorf("stats len = %d", len(stats))
	}
}

func TestCallback(t *testing.T) {
	var called bool
	m := New(WithOnEvent(func(name string, action string) {
		called = true
	}))

	r := m.Register("test")
	r.Allowed()
	if !called {
		t.Error("callback not invoked")
	}
}

func TestFormatStats(t *testing.T) {
	m := New()
	r := m.Register("api")
	r.Allowed()
	r.Throttled()

	output := FormatStats(m.Stats(), false)
	if !strings.Contains(output, "api") {
		t.Errorf("expected 'api' in output: %s", output)
	}
}
