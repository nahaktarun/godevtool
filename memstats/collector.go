package memstats

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nahaktarun/godevtool/internal/color"
	"github.com/nahaktarun/godevtool/internal/ringbuf"
)

// Snapshot wraps runtime.MemStats with computed values and human-readable strings.
type Snapshot struct {
	Timestamp   time.Time
	HeapAlloc   uint64
	HeapSys     uint64
	HeapInuse   uint64
	HeapIdle    uint64
	HeapObjects uint64
	StackInuse  uint64
	NumGC       uint32
	GCPauseAvg  time.Duration
	GCPauseLast time.Duration
	Goroutines  int
	Alloc       uint64 // current bytes allocated
	TotalAlloc  uint64 // cumulative bytes allocated
	Sys         uint64 // total bytes obtained from OS
	Mallocs     uint64
	Frees       uint64
}

// HeapAllocStr returns a human-readable heap allocation string.
func (s Snapshot) HeapAllocStr() string { return formatBytes(s.HeapAlloc) }

// SysStr returns a human-readable system memory string.
func (s Snapshot) SysStr() string { return formatBytes(s.Sys) }

// Collector periodically samples runtime.MemStats.
type Collector struct {
	interval time.Duration
	history  *ringbuf.Buffer[Snapshot]
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewCollector creates a Collector.
// interval is the sampling period; capacity is the max history size.
func NewCollector(interval time.Duration, capacity int) *Collector {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if capacity <= 0 {
		capacity = 100
	}
	return &Collector{
		interval: interval,
		history:  ringbuf.New[Snapshot](capacity),
	}
}

// Start begins background collection.
func (c *Collector) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return
	}
	c.running = true
	c.stopCh = make(chan struct{})
	go c.run()
}

// Stop halts background collection.
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false
	close(c.stopCh)
}

func (c *Collector) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.history.Push(c.sample())

	for {
		select {
		case <-ticker.C:
			c.history.Push(c.sample())
		case <-c.stopCh:
			return
		}
	}
}

// Current returns the current memory snapshot.
func (c *Collector) Current() Snapshot {
	return c.sample()
}

// History returns all collected snapshots.
func (c *Collector) History() []Snapshot {
	return c.history.All()
}

func (c *Collector) sample() Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var gcPauseAvg time.Duration
	if m.NumGC > 0 {
		var total uint64
		count := uint32(256)
		if m.NumGC < count {
			count = m.NumGC
		}
		for i := uint32(0); i < count; i++ {
			total += m.PauseNs[(m.NumGC-1-i)%256]
		}
		gcPauseAvg = time.Duration(total / uint64(count))
	}

	var gcPauseLast time.Duration
	if m.NumGC > 0 {
		gcPauseLast = time.Duration(m.PauseNs[(m.NumGC-1)%256])
	}

	return Snapshot{
		Timestamp:   time.Now(),
		HeapAlloc:   m.HeapAlloc,
		HeapSys:     m.HeapSys,
		HeapInuse:   m.HeapInuse,
		HeapIdle:    m.HeapIdle,
		HeapObjects: m.HeapObjects,
		StackInuse:  m.StackInuse,
		NumGC:       m.NumGC,
		GCPauseAvg:  gcPauseAvg,
		GCPauseLast: gcPauseLast,
		Goroutines:  runtime.NumGoroutine(),
		Alloc:       m.Alloc,
		TotalAlloc:  m.TotalAlloc,
		Sys:         m.Sys,
		Mallocs:     m.Mallocs,
		Frees:       m.Frees,
	}
}

// FormatSnapshot returns a human-readable string of memory stats.
func FormatSnapshot(s Snapshot, colorize bool) string {
	var b strings.Builder

	header := fmt.Sprintf("Memory Stats (at %s)", s.Timestamp.Format("15:04:05"))
	b.WriteString(color.Wrap(header, colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	rows := []struct {
		label string
		value string
	}{
		{"Heap Alloc", formatBytes(s.HeapAlloc)},
		{"Heap Sys", formatBytes(s.HeapSys)},
		{"Heap In-Use", formatBytes(s.HeapInuse)},
		{"Heap Idle", formatBytes(s.HeapIdle)},
		{"Heap Objects", fmt.Sprintf("%d", s.HeapObjects)},
		{"Stack In-Use", formatBytes(s.StackInuse)},
		{"Total Sys", formatBytes(s.Sys)},
		{"Total Alloc", formatBytes(s.TotalAlloc)},
		{"Mallocs", fmt.Sprintf("%d", s.Mallocs)},
		{"Frees", fmt.Sprintf("%d", s.Frees)},
		{"GC Cycles", fmt.Sprintf("%d", s.NumGC)},
		{"GC Pause Avg", s.GCPauseAvg.String()},
		{"GC Pause Last", s.GCPauseLast.String()},
		{"Goroutines", fmt.Sprintf("%d", s.Goroutines)},
	}

	for _, r := range rows {
		b.WriteString(fmt.Sprintf("  %-16s %s\n",
			color.Wrap(r.label, colorize, color.Blue),
			r.value,
		))
	}

	return b.String()
}

// PrintSnapshot writes formatted memory stats to w.
func PrintSnapshot(w io.Writer, s Snapshot, colorize bool) {
	fmt.Fprint(w, FormatSnapshot(s, colorize))
}

func formatBytes(b uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
