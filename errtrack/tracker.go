package errtrack

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// TrackedError represents a single error occurrence.
type TrackedError struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Message   string         `json:"message"`
	Type      string         `json:"type"` // "error" or "panic"
	Stack     string         `json:"stack"`
	GroupKey  string         `json:"group_key"`
	Data      map[string]any `json:"data,omitempty"`
}

// ErrorGroup aggregates similar errors.
type ErrorGroup struct {
	Key         string    `json:"key"`
	Message     string    `json:"message"`
	Type        string    `json:"type"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	SampleStack string   `json:"sample_stack"`
}

// Stats holds error rate statistics.
type Stats struct {
	Total     int          `json:"total"`
	Last1Min  int          `json:"last_1min"`
	Last5Min  int          `json:"last_5min"`
	Last15Min int          `json:"last_15min"`
	ByType    map[string]int `json:"by_type"`
	TopGroups []ErrorGroup `json:"top_groups"`
}

// Tracker captures and groups errors.
type Tracker struct {
	store   *ringbuf.Buffer[TrackedError]
	groups  map[string]*ErrorGroup
	onError func(TrackedError)
	mu      sync.Mutex
	counter uint64
}

// Option configures the Tracker.
type Option func(*Tracker)

// WithCapacity sets the ring buffer capacity (default 500).
func WithCapacity(n int) Option {
	return func(t *Tracker) {
		t.store = ringbuf.New[TrackedError](n)
	}
}

// WithOnError sets a callback invoked for each tracked error.
func WithOnError(fn func(TrackedError)) Option {
	return func(t *Tracker) { t.onError = fn }
}

// New creates a Tracker.
func New(opts ...Option) *Tracker {
	t := &Tracker{
		store:  ringbuf.New[TrackedError](500),
		groups: make(map[string]*ErrorGroup),
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Track records an error occurrence.
func (t *Tracker) Track(err error, data ...map[string]any) {
	if err == nil {
		return
	}

	msg := err.Error()
	groupKey := computeGroupKey(msg, "error")

	// capture stack
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	t.mu.Lock()
	t.counter++
	id := fmt.Sprintf("err%08x", t.counter)

	entry := TrackedError{
		ID:        id,
		Timestamp: time.Now(),
		Message:   msg,
		Type:      "error",
		Stack:     stack,
		GroupKey:  groupKey,
	}
	if len(data) > 0 {
		entry.Data = data[0]
	}

	t.store.Push(entry)
	t.updateGroup(groupKey, msg, "error", stack)
	onError := t.onError
	t.mu.Unlock()

	if onError != nil {
		onError(entry)
	}
}

// TrackPanic records a recovered panic value.
func (t *Tracker) TrackPanic(recovered any, stack string) {
	msg := fmt.Sprintf("%v", recovered)
	groupKey := computeGroupKey(msg, "panic")

	t.mu.Lock()
	t.counter++
	id := fmt.Sprintf("err%08x", t.counter)

	entry := TrackedError{
		ID:        id,
		Timestamp: time.Now(),
		Message:   msg,
		Type:      "panic",
		Stack:     stack,
		GroupKey:  groupKey,
	}

	t.store.Push(entry)
	t.updateGroup(groupKey, msg, "panic", stack)
	onError := t.onError
	t.mu.Unlock()

	if onError != nil {
		onError(entry)
	}
}

// RecoverMiddleware returns an http.Handler that recovers panics.
func (t *Tracker) RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				buf := make([]byte, 8192)
				n := runtime.Stack(buf, false)
				t.TrackPanic(rec, string(buf[:n]))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RecoverFunc wraps a function with panic recovery.
func (t *Tracker) RecoverFunc(fn func()) func() {
	return func() {
		defer func() {
			if rec := recover(); rec != nil {
				buf := make([]byte, 8192)
				n := runtime.Stack(buf, false)
				t.TrackPanic(rec, string(buf[:n]))
			}
		}()
		fn()
	}
}

// Errors returns recent tracked errors.
func (t *Tracker) Errors() []TrackedError {
	return t.store.All()
}

// LastErrors returns the n most recent errors.
func (t *Tracker) LastErrors(n int) []TrackedError {
	return t.store.Last(n)
}

// Groups returns error groups sorted by count descending.
func (t *Tracker) Groups() []ErrorGroup {
	t.mu.Lock()
	defer t.mu.Unlock()

	groups := make([]ErrorGroup, 0, len(t.groups))
	for _, g := range t.groups {
		groups = append(groups, *g)
	}

	// sort by count descending
	for i := 0; i < len(groups); i++ {
		for j := i + 1; j < len(groups); j++ {
			if groups[j].Count > groups[i].Count {
				groups[i], groups[j] = groups[j], groups[i]
			}
		}
	}

	return groups
}

// Stats returns error rate statistics.
func (t *Tracker) Stats() Stats {
	all := t.store.All()
	now := time.Now()

	stats := Stats{
		Total:  len(all),
		ByType: make(map[string]int),
	}

	for _, e := range all {
		stats.ByType[e.Type]++
		age := now.Sub(e.Timestamp)
		if age <= time.Minute {
			stats.Last1Min++
		}
		if age <= 5*time.Minute {
			stats.Last5Min++
		}
		if age <= 15*time.Minute {
			stats.Last15Min++
		}
	}

	groups := t.Groups()
	if len(groups) > 10 {
		groups = groups[:10]
	}
	stats.TopGroups = groups

	return stats
}

// Count returns total tracked errors.
func (t *Tracker) Count() int {
	return t.store.Len()
}

// Clear removes all tracked errors and groups.
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store.Clear()
	t.groups = make(map[string]*ErrorGroup)
}

func (t *Tracker) updateGroup(key, msg, typ, stack string) {
	g, ok := t.groups[key]
	if !ok {
		g = &ErrorGroup{
			Key:         key,
			Message:     msg,
			Type:        typ,
			FirstSeen:   time.Now(),
			SampleStack: stack,
		}
		t.groups[key] = g
	}
	g.Count++
	g.LastSeen = time.Now()
}

// numberRegex strips numbers from error messages for better grouping.
var numberRegex = regexp.MustCompile(`\b\d+\b`)

func computeGroupKey(msg, typ string) string {
	// normalize: strip numbers, lowercase, trim
	normalized := numberRegex.ReplaceAllString(msg, "N")
	normalized = strings.ToLower(strings.TrimSpace(normalized))
	hash := md5.Sum([]byte(typ + ":" + normalized))
	return fmt.Sprintf("%x", hash[:8])
}

// FormatStats returns a human-readable string.
func FormatStats(s Stats, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Error Statistics", colorize, color.Red, color.Bold))
	b.WriteByte('\n')

	b.WriteString(fmt.Sprintf("  %-14s %d\n", color.Wrap("Total", colorize, color.Blue), s.Total))
	b.WriteString(fmt.Sprintf("  %-14s %d\n", color.Wrap("Last 1 min", colorize, color.Blue), s.Last1Min))
	b.WriteString(fmt.Sprintf("  %-14s %d\n", color.Wrap("Last 5 min", colorize, color.Blue), s.Last5Min))
	b.WriteString(fmt.Sprintf("  %-14s %d\n", color.Wrap("Last 15 min", colorize, color.Blue), s.Last15Min))

	if len(s.TopGroups) > 0 {
		b.WriteString(color.Wrap("\n  Top Error Groups:\n", colorize, color.Yellow))
		for _, g := range s.TopGroups {
			b.WriteString(fmt.Sprintf("    [%d] %s (%s)\n",
				g.Count,
				color.Wrap(g.Message, colorize, color.White),
				color.Wrap(g.Type, colorize, color.Gray)))
		}
	}

	return b.String()
}
