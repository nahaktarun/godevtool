package export

import (
	"strings"
	"testing"
)

func TestCaptureJSON(t *testing.T) {
	e := New(DataSource{
		AppName: "test-app",
		Logs:    func() any { return []string{"log1", "log2"} },
		Timers:  func() any { return map[string]int{"op1": 100} },
	})

	data, err := e.CaptureJSON()
	if err != nil {
		t.Fatalf("CaptureJSON failed: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, "test-app") {
		t.Error("expected app_name in JSON")
	}
	if !strings.Contains(s, "log1") {
		t.Error("expected logs in JSON")
	}
	if !strings.Contains(s, "op1") {
		t.Error("expected timers in JSON")
	}
}

func TestCaptureHTML(t *testing.T) {
	e := New(DataSource{
		AppName: "test-app",
		Logs:    func() any { return []string{"hello"} },
	})

	data, err := e.CaptureHTML()
	if err != nil {
		t.Fatalf("CaptureHTML failed: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(s, "test-app") {
		t.Error("expected app name in HTML")
	}
	if !strings.Contains(s, "hello") {
		t.Error("expected log data in HTML")
	}
}

func TestNilProviders(t *testing.T) {
	e := New(DataSource{AppName: "test"})

	data, err := e.CaptureJSON()
	if err != nil {
		t.Fatalf("CaptureJSON failed: %v", err)
	}
	if !strings.Contains(string(data), "test") {
		t.Error("expected app name")
	}
}

func TestWriteTo(t *testing.T) {
	e := New(DataSource{AppName: "test"})

	var b strings.Builder
	err := e.WriteTo(&b, "json")
	if err != nil {
		t.Fatalf("WriteTo json failed: %v", err)
	}
	if b.Len() == 0 {
		t.Error("empty output")
	}

	b.Reset()
	err = e.WriteTo(&b, "html")
	if err != nil {
		t.Fatalf("WriteTo html failed: %v", err)
	}
	if !strings.Contains(b.String(), "<!DOCTYPE") {
		t.Error("expected HTML")
	}

	err = e.WriteTo(&b, "invalid")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}
