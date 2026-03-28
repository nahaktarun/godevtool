package grpcmon

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRecord(t *testing.T) {
	m := New()

	m.Record("/pkg.Service/Method", 50*time.Millisecond, nil, true, "unary")

	if m.Count() != 1 {
		t.Fatalf("count = %d", m.Count())
	}

	calls := m.Calls()
	c := calls[0]

	if c.Method != "/pkg.Service/Method" {
		t.Errorf("method = %q", c.Method)
	}
	if c.Service != "pkg.Service" {
		t.Errorf("service = %q", c.Service)
	}
	if c.Type != "unary" {
		t.Errorf("type = %q", c.Type)
	}
	if c.Duration != 50*time.Millisecond {
		t.Errorf("duration = %v", c.Duration)
	}
	if !c.IsServer {
		t.Error("expected server-side")
	}
	if c.Error != "" {
		t.Errorf("error = %q", c.Error)
	}
}

func TestRecordWithError(t *testing.T) {
	m := New()
	m.Record("/pkg.Svc/M", time.Millisecond, fmt.Errorf("not found"), false, "unary")

	calls := m.Calls()
	if calls[0].Error != "not found" {
		t.Errorf("error = %q", calls[0].Error)
	}
	if calls[0].IsServer {
		t.Error("expected client-side")
	}
}

func TestLastCalls(t *testing.T) {
	m := New()
	for i := 0; i < 10; i++ {
		m.Record("/svc/m", time.Millisecond, nil, true, "unary")
	}

	last := m.LastCalls(3)
	if len(last) != 3 {
		t.Errorf("last calls len = %d", len(last))
	}
}

func TestClear(t *testing.T) {
	m := New()
	m.Record("/svc/m", time.Millisecond, nil, true, "unary")
	m.Clear()
	if m.Count() != 0 {
		t.Errorf("count after clear = %d", m.Count())
	}
}

func TestCallback(t *testing.T) {
	var called bool
	m := New(WithOnCall(func(cl CallLog) {
		called = true
	}))
	m.Record("/svc/m", time.Millisecond, nil, true, "unary")
	if !called {
		t.Error("callback not invoked")
	}
}

func TestSplitMethod(t *testing.T) {
	tests := []struct {
		input   string
		service string
		method  string
	}{
		{"/pkg.Service/Method", "pkg.Service", "Method"},
		{"pkg.Service/Method", "pkg.Service", "Method"},
		{"simple", "simple", ""},
	}
	for _, tc := range tests {
		s, m := splitMethod(tc.input)
		if s != tc.service {
			t.Errorf("splitMethod(%q) service = %q, want %q", tc.input, s, tc.service)
		}
		if m != tc.method {
			t.Errorf("splitMethod(%q) method = %q, want %q", tc.input, m, tc.method)
		}
	}
}

func TestFormatCalls(t *testing.T) {
	m := New()
	m.Record("/svc/Test", 5*time.Millisecond, nil, true, "unary")

	output := FormatCalls(m.Calls(), false)
	if !strings.Contains(output, "Test") {
		t.Errorf("expected method in output: %s", output)
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	m := New()
	interceptor := m.UnaryServerInterceptor()
	if interceptor == nil {
		t.Error("interceptor is nil")
	}
}
