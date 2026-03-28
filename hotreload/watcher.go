package hotreload

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nahaktarun/godevtool/internal/color"
)

// Status represents the watcher state.
type Status string

const (
	StatusIdle       Status = "idle"
	StatusWatching   Status = "watching"
	StatusBuilding   Status = "building"
	StatusRestarting Status = "restarting"
	StatusError      Status = "error"
)

// Event represents a file change event.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
	Op        string    `json:"op"` // "modified"
}

// State holds the current watcher state.
type State struct {
	Status     Status    `json:"status"`
	LastBuild  time.Time `json:"last_build"`
	LastError  string    `json:"last_error,omitempty"`
	BuildCount int       `json:"build_count"`
	WatchDirs  []string  `json:"watch_dirs"`
	BuildCmd   string    `json:"build_cmd"`
}

// Watcher monitors file changes and triggers rebuilds.
type Watcher struct {
	mu         sync.Mutex
	dirs       []string
	extensions []string
	exclude    []string
	buildCmd   string
	runArgs    []string
	debounce   time.Duration
	pollInterval time.Duration
	onEvent    func(Event)
	onBuild    func(State)

	state    State
	stopCh   chan struct{}
	running  bool
	child    *exec.Cmd
	modTimes map[string]time.Time
}

// Option configures the Watcher.
type Option func(*Watcher)

// WithDirs sets directories to watch (default ".").
func WithDirs(dirs ...string) Option {
	return func(w *Watcher) { w.dirs = dirs }
}

// WithExtensions sets file extensions to watch (default ".go").
func WithExtensions(exts ...string) Option {
	return func(w *Watcher) { w.extensions = exts }
}

// WithExclude sets patterns to exclude (e.g. "vendor", "_test.go").
func WithExclude(patterns ...string) Option {
	return func(w *Watcher) { w.exclude = patterns }
}

// WithBuildCmd sets the build command (default "go build -o ./tmp/main .").
func WithBuildCmd(cmd string) Option {
	return func(w *Watcher) { w.buildCmd = cmd }
}

// WithRunArgs sets arguments passed to the built binary.
func WithRunArgs(args ...string) Option {
	return func(w *Watcher) { w.runArgs = args }
}

// WithDebounce sets the debounce delay (default 500ms).
func WithDebounce(d time.Duration) Option {
	return func(w *Watcher) { w.debounce = d }
}

// WithPollInterval sets how often to check for file changes (default 500ms).
func WithPollInterval(d time.Duration) Option {
	return func(w *Watcher) { w.pollInterval = d }
}

// WithOnEvent sets a callback for file change events.
func WithOnEvent(fn func(Event)) Option {
	return func(w *Watcher) { w.onEvent = fn }
}

// WithOnBuild sets a callback for build state changes.
func WithOnBuild(fn func(State)) Option {
	return func(w *Watcher) { w.onBuild = fn }
}

// New creates a Watcher.
func New(opts ...Option) *Watcher {
	w := &Watcher{
		dirs:         []string{"."},
		extensions:   []string{".go"},
		exclude:      []string{"vendor", "node_modules", ".git", "tmp"},
		buildCmd:     "go build -o ./tmp/main .",
		debounce:     500 * time.Millisecond,
		pollInterval: 500 * time.Millisecond,
		modTimes:     make(map[string]time.Time),
	}
	for _, o := range opts {
		o(w)
	}
	w.state.WatchDirs = w.dirs
	w.state.BuildCmd = w.buildCmd
	return w
}

// Start begins watching and auto-rebuilding.
func (w *Watcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("watcher already running")
	}

	// ensure tmp directory exists
	os.MkdirAll("tmp", 0755)

	// take initial snapshot
	w.snapshot()

	w.running = true
	w.stopCh = make(chan struct{})
	w.state.Status = StatusWatching

	go w.poll()
	return nil
}

// Stop halts the watcher and kills any running child process.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()

	w.killChild()
	w.mu.Lock()
	w.state.Status = StatusIdle
	w.mu.Unlock()
}

// State returns the current watcher state.
func (w *Watcher) GetState() State {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// Trigger forces an immediate rebuild/restart.
func (w *Watcher) Trigger() {
	go w.buildAndRestart()
}

func (w *Watcher) poll() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	var lastChange time.Time
	pending := false

	for {
		select {
		case <-ticker.C:
			changed := w.detectChanges()
			if len(changed) > 0 {
				lastChange = time.Now()
				pending = true

				for _, evt := range changed {
					if w.onEvent != nil {
						w.onEvent(evt)
					}
				}
			}

			// debounce: if changes happened and debounce period elapsed
			if pending && time.Since(lastChange) >= w.debounce {
				pending = false
				w.buildAndRestart()
			}
		case <-w.stopCh:
			return
		}
	}
}

func (w *Watcher) detectChanges() []Event {
	var changes []Event

	for _, dir := range w.dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// skip excluded
			for _, excl := range w.exclude {
				if strings.Contains(path, excl) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if info.IsDir() {
				return nil
			}

			// check extension
			matched := false
			for _, ext := range w.extensions {
				if strings.HasSuffix(path, ext) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}

			modTime := info.ModTime()
			w.mu.Lock()
			prev, exists := w.modTimes[path]
			if !exists || modTime.After(prev) {
				w.modTimes[path] = modTime
				if exists { // only report changes after initial snapshot
					changes = append(changes, Event{
						Timestamp: time.Now(),
						Path:      path,
						Op:        "modified",
					})
				}
			}
			w.mu.Unlock()

			return nil
		})
	}

	return changes
}

func (w *Watcher) snapshot() {
	for _, dir := range w.dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			for _, excl := range w.exclude {
				if strings.Contains(path, excl) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if info.IsDir() {
				return nil
			}
			for _, ext := range w.extensions {
				if strings.HasSuffix(path, ext) {
					w.modTimes[path] = info.ModTime()
					break
				}
			}
			return nil
		})
	}
}

func (w *Watcher) buildAndRestart() {
	w.mu.Lock()
	w.state.Status = StatusBuilding
	if w.onBuild != nil {
		w.onBuild(w.state)
	}
	w.mu.Unlock()

	// kill existing child
	w.killChild()

	// run build
	parts := strings.Fields(w.buildCmd)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		w.mu.Lock()
		w.state.Status = StatusError
		w.state.LastError = err.Error()
		if w.onBuild != nil {
			w.onBuild(w.state)
		}
		w.mu.Unlock()
		return
	}

	w.mu.Lock()
	w.state.Status = StatusRestarting
	w.state.BuildCount++
	w.state.LastBuild = time.Now()
	w.state.LastError = ""
	w.mu.Unlock()

	// start the built binary
	binPath := "./tmp/main"
	child := exec.Command(binPath, w.runArgs...)
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := child.Start(); err != nil {
		w.mu.Lock()
		w.state.Status = StatusError
		w.state.LastError = "start failed: " + err.Error()
		if w.onBuild != nil {
			w.onBuild(w.state)
		}
		w.mu.Unlock()
		return
	}

	w.mu.Lock()
	w.child = child
	w.state.Status = StatusWatching
	if w.onBuild != nil {
		w.onBuild(w.state)
	}
	w.mu.Unlock()

	// wait for child to exit in background
	go func() {
		child.Wait()
	}()
}

func (w *Watcher) killChild() {
	w.mu.Lock()
	child := w.child
	w.child = nil
	w.mu.Unlock()

	if child != nil && child.Process != nil {
		child.Process.Kill()
		child.Wait()
	}
}

// FormatState returns a human-readable string.
func FormatState(s State, colorize bool) string {
	var b strings.Builder

	statusColor := color.Green
	switch s.Status {
	case StatusBuilding:
		statusColor = color.Yellow
	case StatusError:
		statusColor = color.Red
	case StatusIdle:
		statusColor = color.Gray
	}

	b.WriteString(color.Wrap("Hot Reload", colorize, color.Cyan, color.Bold))
	b.WriteString(fmt.Sprintf("  [%s]\n",
		color.Wrap(string(s.Status), colorize, statusColor)))
	b.WriteString(fmt.Sprintf("  %-14s %s\n",
		color.Wrap("Build Cmd", colorize, color.Blue), s.BuildCmd))
	b.WriteString(fmt.Sprintf("  %-14s %d\n",
		color.Wrap("Builds", colorize, color.Blue), s.BuildCount))

	if !s.LastBuild.IsZero() {
		b.WriteString(fmt.Sprintf("  %-14s %s\n",
			color.Wrap("Last Build", colorize, color.Blue),
			s.LastBuild.Format("15:04:05")))
	}
	if s.LastError != "" {
		b.WriteString(fmt.Sprintf("  %-14s %s\n",
			color.Wrap("Last Error", colorize, color.Red),
			s.LastError))
	}

	return b.String()
}
