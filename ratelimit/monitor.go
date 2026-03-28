package ratelimit

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
)

// LimiterStats holds statistics for a single rate limiter.
type LimiterStats struct {
	Name         string        `json:"name"`
	Allowed      int64         `json:"allowed"`
	Throttled    int64         `json:"throttled"`
	Queued       int64         `json:"queued"`
	ThrottleRate float64       `json:"throttle_rate"` // throttled / total
	AvgWaitTime  time.Duration `json:"avg_wait_time"`
	QueueDepth   int64         `json:"queue_depth"`
	LastUpdate   time.Time     `json:"last_update"`
}

// Monitor tracks rate limiting decisions across multiple limiters.
type Monitor struct {
	mu       sync.RWMutex
	limiters map[string]*Recorder
	onEvent  func(name string, action string)
}

// Option configures the Monitor.
type Option func(*Monitor)

// WithOnEvent sets a callback invoked on rate limit events.
func WithOnEvent(fn func(name string, action string)) Option {
	return func(m *Monitor) { m.onEvent = fn }
}

// New creates a Monitor.
func New(opts ...Option) *Monitor {
	m := &Monitor{
		limiters: make(map[string]*Recorder),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Register creates tracking for a named rate limiter.
func (m *Monitor) Register(name string) *Recorder {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := &Recorder{
		name:    name,
		onEvent: m.onEvent,
	}
	m.limiters[name] = r
	return r
}

// Stats returns stats for all registered limiters.
func (m *Monitor) Stats() []LimiterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]LimiterStats, 0, len(m.limiters))
	for _, r := range m.limiters {
		result = append(result, r.Stats())
	}
	return result
}

// StatsFor returns stats for a specific limiter.
func (m *Monitor) StatsFor(name string) (LimiterStats, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.limiters[name]
	if !ok {
		return LimiterStats{}, false
	}
	return r.Stats(), true
}

// Count returns the number of registered limiters.
func (m *Monitor) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.limiters)
}

// Recorder is bound to a specific rate limiter. All methods are thread-safe.
type Recorder struct {
	name        string
	allowed     atomic.Int64
	throttled   atomic.Int64
	queued      atomic.Int64
	waitTimeSum atomic.Int64 // nanoseconds
	waitCount   atomic.Int64
	queueDepth  atomic.Int64
	lastUpdate  atomic.Int64 // unix nano
	onEvent     func(string, string)
}

// Allowed records that a request was allowed through.
func (r *Recorder) Allowed() {
	r.allowed.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "allowed")
	}
}

// Throttled records that a request was throttled/rejected.
func (r *Recorder) Throttled() {
	r.throttled.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "throttled")
	}
}

// Queued records that a request was queued with a wait time.
func (r *Recorder) Queued(waitTime time.Duration) {
	r.queued.Add(1)
	r.waitTimeSum.Add(int64(waitTime))
	r.waitCount.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "queued")
	}
}

// SetQueueDepth reports the current queue depth.
func (r *Recorder) SetQueueDepth(n int64) {
	r.queueDepth.Store(n)
	r.touch()
}

// Stats returns current statistics for this limiter.
func (r *Recorder) Stats() LimiterStats {
	allowed := r.allowed.Load()
	throttled := r.throttled.Load()
	total := allowed + throttled

	var throttleRate float64
	if total > 0 {
		throttleRate = float64(throttled) / float64(total)
	}

	var avgWait time.Duration
	if wc := r.waitCount.Load(); wc > 0 {
		avgWait = time.Duration(r.waitTimeSum.Load() / wc)
	}

	var lastUpdate time.Time
	if lu := r.lastUpdate.Load(); lu > 0 {
		lastUpdate = time.Unix(0, lu)
	}

	return LimiterStats{
		Name:         r.name,
		Allowed:      allowed,
		Throttled:    throttled,
		Queued:       r.queued.Load(),
		ThrottleRate: throttleRate,
		AvgWaitTime:  avgWait,
		QueueDepth:   r.queueDepth.Load(),
		LastUpdate:   lastUpdate,
	}
}

func (r *Recorder) touch() {
	r.lastUpdate.Store(time.Now().UnixNano())
}

// FormatStats returns a human-readable string.
func FormatStats(stats []LimiterStats, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Rate Limiter Statistics", colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	if len(stats) == 0 {
		b.WriteString("  No rate limiters registered.\n")
		return b.String()
	}

	for _, s := range stats {
		throttlePct := fmt.Sprintf("%.1f%%", s.ThrottleRate*100)
		b.WriteString(fmt.Sprintf("  %s  throttle-rate=%s  allowed=%d  throttled=%d  queued=%d  avg-wait=%s\n",
			color.Wrap(s.Name, colorize, color.Green),
			color.Wrap(throttlePct, colorize, color.Red),
			s.Allowed, s.Throttled, s.Queued, s.AvgWaitTime))
	}

	return b.String()
}
