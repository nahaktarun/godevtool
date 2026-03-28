package errtrack

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrackError(t *testing.T) {
	tr := New()

	tr.Track(fmt.Errorf("connection refused"))

	if tr.Count() != 1 {
		t.Fatalf("count = %d, want 1", tr.Count())
	}

	errors := tr.Errors()
	if errors[0].Message != "connection refused" {
		t.Errorf("message = %q", errors[0].Message)
	}
	if errors[0].Type != "error" {
		t.Errorf("type = %q, want error", errors[0].Type)
	}
	if errors[0].Stack == "" {
		t.Error("stack should not be empty")
	}
}

func TestTrackNilError(t *testing.T) {
	tr := New()
	tr.Track(nil)
	if tr.Count() != 0 {
		t.Error("nil error should not be tracked")
	}
}

func TestTrackWithData(t *testing.T) {
	tr := New()
	tr.Track(fmt.Errorf("timeout"), map[string]any{"host": "db.local"})

	errors := tr.Errors()
	if errors[0].Data["host"] != "db.local" {
		t.Errorf("data = %v", errors[0].Data)
	}
}

func TestTrackPanic(t *testing.T) {
	tr := New()
	tr.TrackPanic("something broke", "goroutine 1 [running]:\nmain.go:10")

	if tr.Count() != 1 {
		t.Fatalf("count = %d", tr.Count())
	}

	errors := tr.Errors()
	if errors[0].Type != "panic" {
		t.Errorf("type = %q, want panic", errors[0].Type)
	}
	if errors[0].Message != "something broke" {
		t.Errorf("message = %q", errors[0].Message)
	}
}

func TestErrorGrouping(t *testing.T) {
	tr := New()

	// same error message, different numbers — should group
	tr.Track(fmt.Errorf("connection refused on port 5432"))
	tr.Track(fmt.Errorf("connection refused on port 3306"))
	tr.Track(fmt.Errorf("connection refused on port 6379"))

	groups := tr.Groups()
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Count != 3 {
		t.Errorf("group count = %d, want 3", groups[0].Count)
	}
}

func TestDifferentErrors(t *testing.T) {
	tr := New()

	tr.Track(fmt.Errorf("connection refused"))
	tr.Track(fmt.Errorf("timeout exceeded"))

	groups := tr.Groups()
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestStats(t *testing.T) {
	tr := New()

	tr.Track(fmt.Errorf("error 1"))
	tr.Track(fmt.Errorf("error 2"))
	tr.TrackPanic("panic 1", "stack")

	stats := tr.Stats()

	if stats.Total != 3 {
		t.Errorf("total = %d, want 3", stats.Total)
	}
	if stats.Last1Min != 3 {
		t.Errorf("last 1min = %d, want 3", stats.Last1Min)
	}
	if stats.ByType["error"] != 2 {
		t.Errorf("by type error = %d, want 2", stats.ByType["error"])
	}
	if stats.ByType["panic"] != 1 {
		t.Errorf("by type panic = %d, want 1", stats.ByType["panic"])
	}
}

func TestRecoverMiddleware(t *testing.T) {
	tr := New()

	handler := tr.RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if tr.Count() != 1 {
		t.Errorf("count = %d, want 1", tr.Count())
	}
	errors := tr.Errors()
	if errors[0].Type != "panic" {
		t.Errorf("type = %q, want panic", errors[0].Type)
	}
}

func TestRecoverFunc(t *testing.T) {
	tr := New()

	fn := tr.RecoverFunc(func() {
		panic("boom")
	})
	fn() // should not panic

	if tr.Count() != 1 {
		t.Errorf("count = %d, want 1", tr.Count())
	}
}

func TestCallback(t *testing.T) {
	var called bool
	tr := New(WithOnError(func(te TrackedError) {
		called = true
	}))

	tr.Track(fmt.Errorf("test"))
	if !called {
		t.Error("callback not invoked")
	}
}

func TestClear(t *testing.T) {
	tr := New()
	tr.Track(fmt.Errorf("test"))
	tr.Clear()
	if tr.Count() != 0 {
		t.Errorf("count after clear = %d", tr.Count())
	}
	if len(tr.Groups()) != 0 {
		t.Error("groups should be empty after clear")
	}
}

func TestFormatStats(t *testing.T) {
	tr := New()
	tr.Track(fmt.Errorf("test error"))

	output := FormatStats(tr.Stats(), false)
	if !strings.Contains(output, "Error Statistics") {
		t.Errorf("expected header in output: %s", output)
	}
}
