package timeline

import (
	"strings"
	"testing"
	"time"
)

func TestRecord(t *testing.T) {
	tl := New()

	id := tl.Record(CatHTTP, "GET /api/users", map[string]any{"status": 200})

	if id == "" {
		t.Error("expected non-empty event ID")
	}
	if tl.Count() != 1 {
		t.Errorf("count = %d, want 1", tl.Count())
	}

	events := tl.Events()
	evt := events[0]

	if evt.Category != CatHTTP {
		t.Errorf("category = %q, want http", evt.Category)
	}
	if evt.Label != "GET /api/users" {
		t.Errorf("label = %q", evt.Label)
	}
	if evt.IsSpan {
		t.Error("expected point event, not span")
	}
	if evt.Data["status"] != 200 {
		t.Errorf("data status = %v", evt.Data["status"])
	}
}

func TestSpan(t *testing.T) {
	tl := New()

	span := tl.Start(CatDB, "SELECT * FROM users", nil)
	time.Sleep(10 * time.Millisecond)
	span.SetData("rows", 5)
	span.End()

	if tl.Count() != 1 {
		t.Fatalf("count = %d, want 1", tl.Count())
	}

	evt := tl.Events()[0]
	if !evt.IsSpan {
		t.Error("expected span")
	}
	if evt.Duration < 10*time.Millisecond {
		t.Errorf("duration %v < 10ms", evt.Duration)
	}
	if evt.Data["rows"] != 5 {
		t.Errorf("data rows = %v", evt.Data["rows"])
	}
}

func TestSpanDoubleEnd(t *testing.T) {
	tl := New()
	span := tl.Start(CatCustom, "test", nil)
	span.End()
	span.End() // should be no-op

	if tl.Count() != 1 {
		t.Errorf("double end should not create duplicate, count = %d", tl.Count())
	}
}

func TestEventsSince(t *testing.T) {
	tl := New()
	tl.Record(CatHTTP, "old", nil)
	time.Sleep(10 * time.Millisecond)

	cutoff := time.Now()
	time.Sleep(10 * time.Millisecond)
	tl.Record(CatHTTP, "new", nil)

	events := tl.EventsSince(cutoff)
	if len(events) != 1 {
		t.Errorf("EventsSince got %d, want 1", len(events))
	}
	if events[0].Label != "new" {
		t.Errorf("label = %q, want 'new'", events[0].Label)
	}
}

func TestEventsByCategory(t *testing.T) {
	tl := New()
	tl.Record(CatHTTP, "http event", nil)
	tl.Record(CatDB, "db event", nil)
	tl.Record(CatHTTP, "http event 2", nil)

	httpEvents := tl.EventsByCategory(CatHTTP)
	if len(httpEvents) != 2 {
		t.Errorf("got %d http events, want 2", len(httpEvents))
	}
}

func TestLastEvents(t *testing.T) {
	tl := New()
	for i := 0; i < 10; i++ {
		tl.Record(CatCustom, "event", nil)
	}

	last := tl.LastEvents(3)
	if len(last) != 3 {
		t.Errorf("LastEvents(3) len = %d, want 3", len(last))
	}
}

func TestClear(t *testing.T) {
	tl := New()
	tl.Record(CatCustom, "event", nil)
	tl.Clear()
	if tl.Count() != 0 {
		t.Errorf("count after clear = %d", tl.Count())
	}
}

func TestOnEventCallback(t *testing.T) {
	var called bool
	tl := New(WithOnEvent(func(evt Event) {
		called = true
	}))

	tl.Record(CatCustom, "test", nil)
	if !called {
		t.Error("callback not invoked for Record")
	}

	called = false
	span := tl.Start(CatCustom, "span", nil)
	span.End()
	if !called {
		t.Error("callback not invoked for Span.End")
	}
}

func TestCapacity(t *testing.T) {
	tl := New(WithCapacity(5))
	for i := 0; i < 10; i++ {
		tl.Record(CatCustom, "event", nil)
	}
	if tl.Count() != 5 {
		t.Errorf("count = %d, want 5", tl.Count())
	}
}

func TestFormatEvents(t *testing.T) {
	tl := New()
	tl.Record(CatHTTP, "GET /api/users", map[string]any{"status": 200})
	span := tl.Start(CatDB, "SELECT * FROM users", nil)
	span.End()

	output := FormatEvents(tl.Events(), false)
	if !strings.Contains(output, "GET /api/users") {
		t.Errorf("expected event label in output: %s", output)
	}
	if !strings.Contains(output, "SELECT") {
		t.Errorf("expected span label in output: %s", output)
	}
}
