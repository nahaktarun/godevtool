package httptrace

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrapClient(t *testing.T) {
	tr := New()

	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	client := tr.WrapClient(&http.Client{})
	resp, err := client.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()

	if tr.Count() != 1 {
		t.Fatalf("count = %d, want 1", tr.Count())
	}

	traces := tr.Traces()
	trace := traces[0]

	if trace.Method != "GET" {
		t.Errorf("method = %q", trace.Method)
	}
	if trace.StatusCode != 200 {
		t.Errorf("status = %d", trace.StatusCode)
	}
	if trace.Duration <= 0 {
		t.Errorf("duration = %v", trace.Duration)
	}
	if trace.URL == "" {
		t.Error("URL empty")
	}
	if trace.ID == "" {
		t.Error("ID empty")
	}
}

func TestWrapClientNil(t *testing.T) {
	tr := New()
	client := tr.WrapClient(nil)
	if client == nil {
		t.Fatal("WrapClient(nil) should return a valid client")
	}
}

func TestTransport(t *testing.T) {
	tr := New()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: tr.Transport(nil),
	}
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()

	traces := tr.Traces()
	if len(traces) != 1 {
		t.Fatalf("count = %d", len(traces))
	}
	if traces[0].StatusCode != 201 {
		t.Errorf("status = %d", traces[0].StatusCode)
	}
}

func TestMultipleRequests(t *testing.T) {
	tr := New()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := tr.WrapClient(&http.Client{})
	for i := 0; i < 5; i++ {
		resp, _ := client.Get(ts.URL)
		resp.Body.Close()
	}

	if tr.Count() != 5 {
		t.Errorf("count = %d, want 5", tr.Count())
	}
}

func TestLastTraces(t *testing.T) {
	tr := New()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := tr.WrapClient(&http.Client{})
	for i := 0; i < 5; i++ {
		resp, _ := client.Get(ts.URL)
		resp.Body.Close()
	}

	last := tr.LastTraces(2)
	if len(last) != 2 {
		t.Errorf("LastTraces(2) len = %d", len(last))
	}
}

func TestErrorTrace(t *testing.T) {
	tr := New()

	client := tr.WrapClient(&http.Client{})
	_, err := client.Get("http://127.0.0.1:1") // should fail
	if err == nil {
		t.Fatal("expected error")
	}

	traces := tr.Traces()
	if len(traces) != 1 {
		t.Fatalf("count = %d", len(traces))
	}
	if traces[0].Error == "" {
		t.Error("expected error in trace")
	}
}

func TestCallback(t *testing.T) {
	var called bool
	tr := New(WithOnTrace(func(rt RequestTrace) {
		called = true
	}))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := tr.WrapClient(&http.Client{})
	resp, _ := client.Get(ts.URL)
	resp.Body.Close()

	if !called {
		t.Error("callback not invoked")
	}
}

func TestClear(t *testing.T) {
	tr := New()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := tr.WrapClient(&http.Client{})
	resp, _ := client.Get(ts.URL)
	resp.Body.Close()

	tr.Clear()
	if tr.Count() != 0 {
		t.Errorf("count after clear = %d", tr.Count())
	}
}
