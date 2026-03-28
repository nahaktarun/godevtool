package profiler

import (
	"strings"
	"testing"
	"time"
)

func TestCaptureHeap(t *testing.T) {
	p := New()

	prof, err := p.CaptureHeap()
	if err != nil {
		t.Fatalf("CaptureHeap failed: %v", err)
	}

	if prof.Type != "heap" {
		t.Errorf("type = %q, want heap", prof.Type)
	}
	if prof.Size == 0 {
		t.Error("size = 0")
	}
	if prof.ID == "" {
		t.Error("ID empty")
	}
	if p.Count() != 1 {
		t.Errorf("count = %d, want 1", p.Count())
	}
}

func TestCaptureCPU(t *testing.T) {
	p := New()

	prof, err := p.CaptureCPU(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("CaptureCPU failed: %v", err)
	}

	if prof.Type != "cpu" {
		t.Errorf("type = %q, want cpu", prof.Type)
	}
	if prof.Duration < 100*time.Millisecond {
		t.Errorf("duration = %v, expected >= 100ms", prof.Duration)
	}
	if prof.Size == 0 {
		t.Error("size = 0")
	}
}

func TestCaptureCPUConcurrent(t *testing.T) {
	p := New()

	// Start one CPU profile
	go func() {
		p.CaptureCPU(200 * time.Millisecond)
	}()
	time.Sleep(50 * time.Millisecond)

	// Second should fail
	_, err := p.CaptureCPU(100 * time.Millisecond)
	if err == nil {
		t.Error("expected error for concurrent CPU capture")
	}

	time.Sleep(200 * time.Millisecond) // wait for first to finish
}

func TestCaptureGoroutine(t *testing.T) {
	p := New()

	prof, err := p.CaptureGoroutine()
	if err != nil {
		t.Fatalf("CaptureGoroutine failed: %v", err)
	}

	if prof.Type != "goroutine" {
		t.Errorf("type = %q", prof.Type)
	}
	if prof.Size == 0 {
		t.Error("size = 0")
	}
}

func TestProfileData(t *testing.T) {
	p := New()

	prof, _ := p.CaptureHeap()
	data, ok := p.ProfileData(prof.ID)
	if !ok {
		t.Fatal("profile not found")
	}
	if len(data) == 0 {
		t.Error("data is empty")
	}

	_, ok = p.ProfileData("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestProfiles(t *testing.T) {
	p := New()

	p.CaptureHeap()
	p.CaptureGoroutine()

	profiles := p.Profiles()
	if len(profiles) != 2 {
		t.Errorf("profiles count = %d, want 2", len(profiles))
	}
}

func TestIsCapturing(t *testing.T) {
	p := New()

	if p.IsCapturing() {
		t.Error("should not be capturing initially")
	}
}

func TestFormatProfiles(t *testing.T) {
	p := New()
	p.CaptureHeap()

	output := FormatProfiles(p.Profiles(), false)
	if !strings.Contains(output, "heap") {
		t.Errorf("expected heap in output: %s", output)
	}
}

func TestOnCapture(t *testing.T) {
	var called bool
	p := New(WithOnCapture(func(prof Profile) {
		called = true
	}))

	p.CaptureHeap()
	if !called {
		t.Error("callback not invoked")
	}
}
