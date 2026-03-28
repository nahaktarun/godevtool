package timeline

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// Category constants for common event types.
const (
	CatHTTP      = "http"
	CatDB        = "db"
	CatCustom    = "custom"
	CatGC        = "gc"
	CatGoroutine = "goroutine"
	CatTimer     = "timer"
	CatLog       = "log"
)

// Event represents a timestamped occurrence in the application lifecycle.
type Event struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	EndTime   time.Time      `json:"end_time,omitempty"`
	Category  string         `json:"category"`
	Label     string         `json:"label"`
	Duration  time.Duration  `json:"duration,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	IsSpan    bool           `json:"is_span"` // true if it has duration
}

// Span represents an in-progress event that can be ended.
type Span struct {
	event    *Event
	timeline *Timeline
	ended    bool
	mu       sync.Mutex
}

// End completes the span, recording its duration.
func (s *Span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ended = true
	s.event.EndTime = time.Now()
	s.event.Duration = s.event.EndTime.Sub(s.event.Timestamp)
	s.event.IsSpan = true
	s.timeline.store.Push(*s.event)

	if s.timeline.onEvent != nil {
		s.timeline.onEvent(*s.event)
	}
}

// SetData adds or updates data on the span before ending.
func (s *Span) SetData(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.event.Data == nil {
		s.event.Data = make(map[string]any)
	}
	s.event.Data[key] = value
}

// Timeline records events in chronological order.
type Timeline struct {
	store   *ringbuf.Buffer[Event]
	onEvent func(Event)
	mu      sync.Mutex
	counter uint64
}

// Option configures the Timeline.
type Option func(*Timeline)

// WithCapacity sets the ring buffer capacity (default 1000).
func WithCapacity(n int) Option {
	return func(t *Timeline) {
		t.store = ringbuf.New[Event](n)
	}
}

// WithOnEvent sets a callback invoked for each recorded event.
func WithOnEvent(fn func(Event)) Option {
	return func(t *Timeline) { t.onEvent = fn }
}

// New creates a Timeline.
func New(opts ...Option) *Timeline {
	t := &Timeline{
		store: ringbuf.New[Event](1000),
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Record adds a point-in-time event. Returns the event ID.
func (t *Timeline) Record(category, label string, data map[string]any) string {
	t.mu.Lock()
	t.counter++
	id := fmt.Sprintf("evt%08x", t.counter)
	t.mu.Unlock()

	evt := Event{
		ID:        id,
		Timestamp: time.Now(),
		Category:  category,
		Label:     label,
		Data:      data,
		IsSpan:    false,
	}

	t.store.Push(evt)

	if t.onEvent != nil {
		t.onEvent(evt)
	}

	return id
}

// Start begins a span (an event with duration). Call span.End() to complete it.
// Typically used with defer:
//
//	span := timeline.Start("http", "GET /api/users", nil)
//	defer span.End()
func (t *Timeline) Start(category, label string, data map[string]any) *Span {
	t.mu.Lock()
	t.counter++
	id := fmt.Sprintf("evt%08x", t.counter)
	t.mu.Unlock()

	evt := &Event{
		ID:        id,
		Timestamp: time.Now(),
		Category:  category,
		Label:     label,
		Data:      data,
		IsSpan:    true,
	}

	return &Span{
		event:    evt,
		timeline: t,
	}
}

// Events returns all events.
func (t *Timeline) Events() []Event {
	return t.store.All()
}

// EventsSince returns events after the given time.
func (t *Timeline) EventsSince(since time.Time) []Event {
	all := t.store.All()
	var result []Event
	for _, e := range all {
		if e.Timestamp.After(since) {
			result = append(result, e)
		}
	}
	return result
}

// EventsByCategory returns events matching the given category.
func (t *Timeline) EventsByCategory(category string) []Event {
	all := t.store.All()
	var result []Event
	for _, e := range all {
		if e.Category == category {
			result = append(result, e)
		}
	}
	return result
}

// LastEvents returns the n most recent events.
func (t *Timeline) LastEvents(n int) []Event {
	return t.store.Last(n)
}

// Clear removes all events.
func (t *Timeline) Clear() {
	t.store.Clear()
}

// Count returns the number of stored events.
func (t *Timeline) Count() int {
	return t.store.Len()
}

// FormatEvents returns a human-readable string of recent events.
func FormatEvents(events []Event, colorize bool) string {
	var b strings.Builder

	catColors := map[string]color.Code{
		CatHTTP:      color.Green,
		CatDB:        color.Cyan,
		CatCustom:    color.White,
		CatGC:        color.Yellow,
		CatGoroutine: color.Yellow,
		CatTimer:     color.Blue,
		CatLog:       color.Gray,
	}

	for _, e := range events {
		ts := e.Timestamp.Format("15:04:05.000")
		b.WriteString(color.Wrap(ts, colorize, color.Gray))
		b.WriteByte(' ')

		catColor, ok := catColors[e.Category]
		if !ok {
			catColor = color.White
		}
		cat := fmt.Sprintf("%-10s", e.Category)
		b.WriteString(color.Wrap(cat, colorize, catColor))
		b.WriteByte(' ')

		b.WriteString(e.Label)

		if e.IsSpan && e.Duration > 0 {
			b.WriteByte(' ')
			b.WriteString(color.Wrap(e.Duration.String(), colorize, color.Cyan))
		}

		if len(e.Data) > 0 {
			b.WriteByte(' ')
			for k, v := range e.Data {
				b.WriteString(color.Wrap(k, colorize, color.Blue))
				b.WriteByte('=')
				b.WriteString(fmt.Sprintf("%v", v))
				b.WriteByte(' ')
			}
		}

		b.WriteByte('\n')
	}

	return b.String()
}
