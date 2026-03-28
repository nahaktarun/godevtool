package cachemon

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
)

// CacheStats holds aggregate statistics for a single cache.
type CacheStats struct {
	Name       string    `json:"name"`
	Hits       int64     `json:"hits"`
	Misses     int64     `json:"misses"`
	Sets       int64     `json:"sets"`
	Evictions  int64     `json:"evictions"`
	Deletes    int64     `json:"deletes"`
	HitRate    float64   `json:"hit_rate"` // hits / (hits + misses)
	Size       int64     `json:"size"`     // current item count
	LastUpdate time.Time `json:"last_update"`
}

// Monitor tracks cache metrics across multiple named caches.
type Monitor struct {
	mu      sync.RWMutex
	caches  map[string]*Recorder
	onEvent func(name string, evt string)
}

// Option configures the Monitor.
type Option func(*Monitor)

// WithOnEvent sets a callback invoked on cache events.
func WithOnEvent(fn func(name string, evt string)) Option {
	return func(m *Monitor) { m.onEvent = fn }
}

// New creates a Monitor.
func New(opts ...Option) *Monitor {
	m := &Monitor{
		caches: make(map[string]*Recorder),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Register creates a named cache tracker. Returns a Recorder for that cache.
func (m *Monitor) Register(name string) *Recorder {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := &Recorder{
		name:    name,
		onEvent: m.onEvent,
	}
	m.caches[name] = r
	return r
}

// Stats returns stats for all registered caches.
func (m *Monitor) Stats() []CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]CacheStats, 0, len(m.caches))
	for _, r := range m.caches {
		result = append(result, r.Stats())
	}
	return result
}

// StatsFor returns stats for a specific cache.
func (m *Monitor) StatsFor(name string) (CacheStats, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.caches[name]
	if !ok {
		return CacheStats{}, false
	}
	return r.Stats(), true
}

// Names returns all registered cache names.
func (m *Monitor) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.caches))
	for name := range m.caches {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered caches.
func (m *Monitor) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.caches)
}

// Recorder is bound to a specific cache name. All methods are thread-safe.
type Recorder struct {
	name       string
	hits       atomic.Int64
	misses     atomic.Int64
	sets       atomic.Int64
	evictions  atomic.Int64
	deletes    atomic.Int64
	size       atomic.Int64
	lastUpdate atomic.Int64 // unix nano
	onEvent    func(string, string)
}

// Hit records a cache hit.
func (r *Recorder) Hit() {
	r.hits.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "hit")
	}
}

// Miss records a cache miss.
func (r *Recorder) Miss() {
	r.misses.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "miss")
	}
}

// Set records a cache set operation.
func (r *Recorder) Set() {
	r.sets.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "set")
	}
}

// Evict records a cache eviction.
func (r *Recorder) Evict() {
	r.evictions.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "eviction")
	}
}

// Delete records a cache delete.
func (r *Recorder) Delete() {
	r.deletes.Add(1)
	r.touch()
	if r.onEvent != nil {
		r.onEvent(r.name, "delete")
	}
}

// SetSize reports the current number of items in the cache.
func (r *Recorder) SetSize(n int64) {
	r.size.Store(n)
	r.touch()
}

// Stats returns current statistics for this cache.
func (r *Recorder) Stats() CacheStats {
	hits := r.hits.Load()
	misses := r.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	var lastUpdate time.Time
	if lu := r.lastUpdate.Load(); lu > 0 {
		lastUpdate = time.Unix(0, lu)
	}

	return CacheStats{
		Name:       r.name,
		Hits:       hits,
		Misses:     misses,
		Sets:       r.sets.Load(),
		Evictions:  r.evictions.Load(),
		Deletes:    r.deletes.Load(),
		HitRate:    hitRate,
		Size:       r.size.Load(),
		LastUpdate: lastUpdate,
	}
}

func (r *Recorder) touch() {
	r.lastUpdate.Store(time.Now().UnixNano())
}

// FormatStats returns a human-readable string of all cache stats.
func FormatStats(stats []CacheStats, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Cache Statistics", colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	if len(stats) == 0 {
		b.WriteString("  No caches registered.\n")
		return b.String()
	}

	for _, s := range stats {
		hitPct := fmt.Sprintf("%.1f%%", s.HitRate*100)
		b.WriteString(fmt.Sprintf("  %s  hit-rate=%s  hits=%d  misses=%d  sets=%d  evictions=%d  size=%d\n",
			color.Wrap(s.Name, colorize, color.Green),
			color.Wrap(hitPct, colorize, color.Yellow),
			s.Hits, s.Misses, s.Sets, s.Evictions, s.Size))
	}

	return b.String()
}
