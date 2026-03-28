package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInspectorCaptures(t *testing.T) {
	ins := New()

	handler := ins.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest("GET", "/api/users?page=1", nil)
	req.Header.Set("X-Request-ID", "test-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ins.Count() != 1 {
		t.Fatalf("count = %d, want 1", ins.Count())
	}

	logs := ins.Requests()
	entry := logs[0]

	if entry.Method != "GET" {
		t.Errorf("method = %q, want GET", entry.Method)
	}
	if entry.Path != "/api/users" {
		t.Errorf("path = %q, want /api/users", entry.Path)
	}
	if entry.Query != "page=1" {
		t.Errorf("query = %q, want page=1", entry.Query)
	}
	if entry.StatusCode != 200 {
		t.Errorf("status = %d, want 200", entry.StatusCode)
	}
	if string(entry.ResponseBody) != `{"status":"ok"}` {
		t.Errorf("response body = %q", string(entry.ResponseBody))
	}
	if entry.RequestHeaders.Get("X-Request-ID") != "test-123" {
		t.Error("request header X-Request-ID not captured")
	}
}

func TestInspectorCapturesRequestBody(t *testing.T) {
	ins := New()

	handler := ins.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Write(body) // echo back
	}))

	body := `{"name":"alice"}`
	req := httptest.NewRequest("POST", "/api/users", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	logs := ins.Requests()
	entry := logs[0]

	if string(entry.RequestBody) != body {
		t.Errorf("request body = %q, want %q", string(entry.RequestBody), body)
	}
	if entry.Method != "POST" {
		t.Errorf("method = %q, want POST", entry.Method)
	}
}

func TestInspectorCallback(t *testing.T) {
	var called bool
	ins := New(WithOnLog(func(rl RequestLog) {
		called = true
		if rl.StatusCode != 201 {
			t.Errorf("callback status = %d, want 201", rl.StatusCode)
		}
	}))

	handler := ins.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/api/items", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Error("callback was not invoked")
	}
}

func TestInspectorClear(t *testing.T) {
	ins := New()
	handler := ins.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if ins.Count() != 2 {
		t.Fatalf("count = %d, want 2", ins.Count())
	}

	ins.Clear()
	if ins.Count() != 0 {
		t.Errorf("count after clear = %d, want 0", ins.Count())
	}
}

func TestInspectorLastRequests(t *testing.T) {
	ins := New()
	handler := ins.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	for i := 0; i < 5; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}

	last := ins.LastRequests(2)
	if len(last) != 2 {
		t.Errorf("LastRequests(2) len = %d, want 2", len(last))
	}
}

func TestHandlerFunc(t *testing.T) {
	ins := New()

	handler := ins.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler(httptest.NewRecorder(), httptest.NewRequest("GET", "/test", nil))

	if ins.Count() != 1 {
		t.Errorf("count = %d, want 1", ins.Count())
	}
}
