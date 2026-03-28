package profiler

import (
	"bytes"
	"fmt"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// Profile represents a captured profile.
type Profile struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"` // "cpu", "heap", "goroutine", "mutex", "block", "allocs", "threadcreate"
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
	Size      int           `json:"size"` // bytes
	data      []byte
}

// Profiler manages pprof profile capture.
type Profiler struct {
	mu         sync.Mutex
	capturing  bool // true if CPU profile is in progress
	store      *ringbuf.Buffer[Profile]
	profiles   map[string]*Profile
	counter    uint64
	onCapture  func(Profile)
}

// Option configures the Profiler.
type Option func(*Profiler)

// WithOnCapture sets a callback invoked after each capture.
func WithOnCapture(fn func(Profile)) Option {
	return func(p *Profiler) { p.onCapture = fn }
}

// New creates a Profiler.
func New(opts ...Option) *Profiler {
	p := &Profiler{
		store:    ringbuf.New[Profile](50),
		profiles: make(map[string]*Profile),
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// CaptureHeap captures a heap profile snapshot.
func (p *Profiler) CaptureHeap() (*Profile, error) {
	var buf bytes.Buffer
	if err := pprof.Lookup("heap").WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("capture heap: %w", err)
	}
	return p.saveProfile("heap", 0, buf.Bytes()), nil
}

// CaptureCPU captures a CPU profile for the given duration.
// This is blocking for the duration.
func (p *Profiler) CaptureCPU(duration time.Duration) (*Profile, error) {
	p.mu.Lock()
	if p.capturing {
		p.mu.Unlock()
		return nil, fmt.Errorf("CPU profile already in progress")
	}
	p.capturing = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.capturing = false
		p.mu.Unlock()
	}()

	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, fmt.Errorf("start CPU profile: %w", err)
	}
	time.Sleep(duration)
	pprof.StopCPUProfile()

	return p.saveProfile("cpu", duration, buf.Bytes()), nil
}

// CaptureGoroutine captures a goroutine dump.
func (p *Profiler) CaptureGoroutine() (*Profile, error) {
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 1); err != nil {
		return nil, fmt.Errorf("capture goroutine: %w", err)
	}
	return p.saveProfile("goroutine", 0, buf.Bytes()), nil
}

// CaptureMutex captures a mutex contention profile.
func (p *Profiler) CaptureMutex() (*Profile, error) {
	var buf bytes.Buffer
	prof := pprof.Lookup("mutex")
	if prof == nil {
		return nil, fmt.Errorf("mutex profile not available")
	}
	if err := prof.WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("capture mutex: %w", err)
	}
	return p.saveProfile("mutex", 0, buf.Bytes()), nil
}

// CaptureBlock captures a block (contention) profile.
func (p *Profiler) CaptureBlock() (*Profile, error) {
	var buf bytes.Buffer
	prof := pprof.Lookup("block")
	if prof == nil {
		return nil, fmt.Errorf("block profile not available")
	}
	if err := prof.WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("capture block: %w", err)
	}
	return p.saveProfile("block", 0, buf.Bytes()), nil
}

// CaptureAllocs captures an allocation profile.
func (p *Profiler) CaptureAllocs() (*Profile, error) {
	var buf bytes.Buffer
	prof := pprof.Lookup("allocs")
	if prof == nil {
		return nil, fmt.Errorf("allocs profile not available")
	}
	if err := prof.WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("capture allocs: %w", err)
	}
	return p.saveProfile("allocs", 0, buf.Bytes()), nil
}

// IsCapturing returns true if a CPU profile is currently being captured.
func (p *Profiler) IsCapturing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.capturing
}

// Profiles returns metadata for all captured profiles (no data).
func (p *Profiler) Profiles() []Profile {
	return p.store.All()
}

// ProfileData returns the raw pprof bytes for a profile by ID.
func (p *Profiler) ProfileData(id string) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	prof, ok := p.profiles[id]
	if !ok {
		return nil, false
	}
	return prof.data, true
}

// Count returns the number of stored profiles.
func (p *Profiler) Count() int {
	return p.store.Len()
}

func (p *Profiler) saveProfile(typ string, duration time.Duration, data []byte) *Profile {
	p.mu.Lock()
	p.counter++
	id := fmt.Sprintf("prof%08x", p.counter)
	prof := &Profile{
		ID:        id,
		Type:      typ,
		Timestamp: time.Now(),
		Duration:  duration,
		Size:      len(data),
		data:      data,
	}
	p.profiles[id] = prof
	p.store.Push(*prof)
	onCapture := p.onCapture
	p.mu.Unlock()

	if onCapture != nil {
		onCapture(*prof)
	}

	return prof
}

// FormatProfiles returns a human-readable string of captured profiles.
func FormatProfiles(profiles []Profile, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Captured Profiles", colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	if len(profiles) == 0 {
		b.WriteString("  No profiles captured yet.\n")
		return b.String()
	}

	for _, p := range profiles {
		dur := ""
		if p.Duration > 0 {
			dur = fmt.Sprintf(" (%s)", p.Duration)
		}
		b.WriteString(fmt.Sprintf("  %s  %-10s  %s%s\n",
			color.Wrap(p.Timestamp.Format("15:04:05"), colorize, color.Gray),
			color.Wrap(p.Type, colorize, color.Green),
			color.Wrap(formatBytes(p.Size), colorize, color.White),
			dur))
	}

	return b.String()
}

func formatBytes(b int) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
