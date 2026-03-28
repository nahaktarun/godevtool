package hotreload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	w := New()

	if len(w.dirs) != 1 || w.dirs[0] != "." {
		t.Errorf("dirs = %v", w.dirs)
	}
	if len(w.extensions) != 1 || w.extensions[0] != ".go" {
		t.Errorf("extensions = %v", w.extensions)
	}
	if w.debounce != 500*time.Millisecond {
		t.Errorf("debounce = %v", w.debounce)
	}
}

func TestNewWithOptions(t *testing.T) {
	w := New(
		WithDirs("src", "pkg"),
		WithExtensions(".go", ".html"),
		WithExclude("vendor"),
		WithBuildCmd("go build -o ./bin/app ."),
		WithDebounce(time.Second),
	)

	if len(w.dirs) != 2 {
		t.Errorf("dirs = %v", w.dirs)
	}
	if len(w.extensions) != 2 {
		t.Errorf("extensions = %v", w.extensions)
	}
	if w.buildCmd != "go build -o ./bin/app ." {
		t.Errorf("buildCmd = %q", w.buildCmd)
	}
}

func TestGetState(t *testing.T) {
	w := New()
	state := w.GetState()

	if state.Status != "" && state.Status != StatusIdle {
		// before Start, status could be zero or idle
	}
	if state.BuildCount != 0 {
		t.Errorf("build count = %d", state.BuildCount)
	}
}

func TestDetectChanges(t *testing.T) {
	dir := t.TempDir()

	// create initial file
	testFile := filepath.Join(dir, "main.go")
	os.WriteFile(testFile, []byte("package main"), 0644)

	w := New(WithDirs(dir))
	w.snapshot() // take initial snapshot

	// modify file
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(testFile, []byte("package main\n// changed"), 0644)

	changes := w.detectChanges()
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(changes))
	}
	if changes[0].Op != "modified" {
		t.Errorf("op = %q", changes[0].Op)
	}
}

func TestDetectNoChanges(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	w := New(WithDirs(dir))
	w.snapshot()

	changes := w.detectChanges()
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d", len(changes))
	}
}

func TestExcludePatterns(t *testing.T) {
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "vendor")
	os.MkdirAll(vendorDir, 0755)
	os.WriteFile(filepath.Join(vendorDir, "dep.go"), []byte("package dep"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	w := New(WithDirs(dir), WithExclude("vendor"))
	w.snapshot()

	// only main.go should be tracked
	if _, ok := w.modTimes[filepath.Join(vendorDir, "dep.go")]; ok {
		t.Error("vendor file should be excluded")
	}
}

func TestExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0644)

	w := New(WithDirs(dir), WithExtensions(".go"))
	w.snapshot()

	if _, ok := w.modTimes[filepath.Join(dir, "style.css")]; ok {
		t.Error("css file should not be tracked")
	}
}

func TestOnEventCallback(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "main.go")
	os.WriteFile(testFile, []byte("package main"), 0644)

	var events []Event
	w := New(
		WithDirs(dir),
		WithOnEvent(func(evt Event) {
			events = append(events, evt)
		}),
	)
	w.snapshot()

	time.Sleep(50 * time.Millisecond)
	os.WriteFile(testFile, []byte("package main\n// v2"), 0644)

	changes := w.detectChanges()
	for _, evt := range changes {
		if w.onEvent != nil {
			w.onEvent(evt)
		}
	}

	if len(events) != 1 {
		t.Errorf("events = %d", len(events))
	}
}

func TestFormatState(t *testing.T) {
	state := State{
		Status:     StatusWatching,
		BuildCount: 3,
		BuildCmd:   "go build .",
	}
	output := FormatState(state, false)
	if !strings.Contains(output, "watching") {
		t.Errorf("expected 'watching' in output: %s", output)
	}
}
