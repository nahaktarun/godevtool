package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tarunnahak/godevtool/log"
	"github.com/tarunnahak/godevtool/middleware"
	"github.com/tarunnahak/godevtool/timer"
)

func newTestServer() *Server {
	logger := log.New(nil, log.LevelDebug, false, "15:04:05")
	logger.Info("test log", "key", "value")

	ins := middleware.New()
	handler := ins.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/test", nil))

	report := timer.NewReport()
	report.Record("test-timer", 50000000) // 50ms in nanoseconds

	providers := DataProviders{
		Logger:     logger,
		Middleware: ins,
		Timer:      report,
	}

	return NewServer(":0", providers)
}

func TestHandleLogs(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var logs []log.LogEntry
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected at least one log entry")
	}
	if logs[0].Message != "test log" {
		t.Errorf("message = %q, want 'test log'", logs[0].Message)
	}
}

func TestHandleRequests(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/api/requests", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var reqs []middleware.RequestLog
	if err := json.NewDecoder(w.Body).Decode(&reqs); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(reqs) == 0 {
		t.Error("expected at least one request")
	}
}

func TestHandleTimers(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/api/timers", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var timers []timer.Stats
	if err := json.NewDecoder(w.Body).Decode(&timers); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(timers) == 0 {
		t.Error("expected at least one timer")
	}
	if timers[0].Label != "test-timer" {
		t.Errorf("label = %q, want 'test-timer'", timers[0].Label)
	}
}

func TestHandleOverview(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/api/overview", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var data overviewData
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if data.LogCount == 0 {
		t.Error("expected non-zero log count")
	}
	if data.RequestCount == 0 {
		t.Error("expected non-zero request count")
	}
}

func TestStaticFileServing(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "godevtool") {
		t.Error("expected dashboard HTML to contain 'godevtool'")
	}
}

func TestHandleGoroutinesNilProvider(t *testing.T) {
	s := NewServer(":0", DataProviders{})
	req := httptest.NewRequest("GET", "/api/goroutines", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestHandleMemStatsNilProvider(t *testing.T) {
	s := NewServer(":0", DataProviders{})
	req := httptest.NewRequest("GET", "/api/memstats", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestHubBroadcast(t *testing.T) {
	h := newHub()

	c := &client{send: make(chan []byte, 10)}
	h.register(c)

	h.Broadcast(Event{Type: "test", Data: "hello"})

	select {
	case msg := <-c.send:
		if !strings.Contains(string(msg), "test") {
			t.Errorf("broadcast message = %q, expected to contain 'test'", string(msg))
		}
	default:
		t.Error("expected a message from broadcast")
	}

	h.unregister(c)
	if h.clientCount() != 0 {
		t.Errorf("client count after unregister = %d, want 0", h.clientCount())
	}
}

func TestServerStartStop(t *testing.T) {
	s := NewServer(":0", DataProviders{})
	if err := s.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
