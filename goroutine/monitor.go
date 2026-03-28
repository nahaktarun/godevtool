package goroutine

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// GoroutineInfo describes a single goroutine.
type GoroutineInfo struct {
	ID       int
	State    string
	Function string
	Stack    string
}

// Snapshot represents goroutine state at a point in time.
type Snapshot struct {
	Timestamp  time.Time
	Count      int
	Goroutines []GoroutineInfo
}

// Monitor tracks goroutine counts over time and can detect leaks.
type Monitor struct {
	interval time.Duration
	history  *ringbuf.Buffer[Snapshot]
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewMonitor creates a Monitor that samples every interval.
// History keeps the last 100 snapshots by default.
func NewMonitor(interval time.Duration) *Monitor {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Monitor{
		interval: interval,
		history:  ringbuf.New[Snapshot](100),
	}
}

// Start begins background monitoring.
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.run()
}

// Stop halts background monitoring.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
}

func (m *Monitor) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// take initial snapshot
	m.history.Push(m.takeSnapshot())

	for {
		select {
		case <-ticker.C:
			m.history.Push(m.takeSnapshot())
		case <-m.stopCh:
			return
		}
	}
}

// Current returns the current goroutine snapshot.
func (m *Monitor) Current() Snapshot {
	return m.takeSnapshot()
}

// History returns recent snapshots.
func (m *Monitor) History() []Snapshot {
	return m.history.All()
}

// Count returns the current number of goroutines.
func (m *Monitor) Count() int {
	return runtime.NumGoroutine()
}

// LeakCheck compares the first and last snapshots in history.
// It returns goroutines present in the latest snapshot that were not
// present in the earliest snapshot (by function name).
func (m *Monitor) LeakCheck() []GoroutineInfo {
	all := m.history.All()
	if len(all) < 2 {
		return nil
	}

	first := all[0]
	last := all[len(all)-1]

	// count functions in the first snapshot
	baseline := make(map[string]int)
	for _, g := range first.Goroutines {
		baseline[g.Function]++
	}

	// find new goroutines
	current := make(map[string]int)
	for _, g := range last.Goroutines {
		current[g.Function]++
	}

	var suspects []GoroutineInfo
	for _, g := range last.Goroutines {
		if current[g.Function] > baseline[g.Function] {
			suspects = append(suspects, g)
			// decrement to avoid duplicates
			current[g.Function]--
		}
	}
	return suspects
}

func (m *Monitor) takeSnapshot() Snapshot {
	buf := make([]byte, 1<<20) // 1MB
	n := runtime.Stack(buf, true)
	raw := string(buf[:n])

	goroutines := parseGoroutines(raw)

	return Snapshot{
		Timestamp:  time.Now(),
		Count:      runtime.NumGoroutine(),
		Goroutines: goroutines,
	}
}

func parseGoroutines(raw string) []GoroutineInfo {
	blocks := strings.Split(raw, "\n\n")
	var goroutines []GoroutineInfo

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}

		g := GoroutineInfo{Stack: block}

		// parse header: "goroutine 1 [running]:"
		header := lines[0]
		if strings.HasPrefix(header, "goroutine ") {
			var id int
			var state string
			fmt.Sscanf(header, "goroutine %d [%s", &id, &state)
			g.ID = id
			// extract state from brackets
			if start := strings.Index(header, "["); start >= 0 {
				if end := strings.Index(header, "]"); end > start {
					g.State = header[start+1 : end]
				}
			}
		}

		// extract top function name (second line)
		if len(lines) > 1 {
			funcLine := strings.TrimSpace(lines[1])
			// remove arguments like "(0x1234, 0x5678)"
			if idx := strings.Index(funcLine, "("); idx >= 0 {
				funcLine = funcLine[:idx]
			}
			g.Function = funcLine
		}

		goroutines = append(goroutines, g)
	}

	return goroutines
}

// FormatSnapshot returns a human-readable string of a snapshot.
func FormatSnapshot(s Snapshot, colorize bool) string {
	var b strings.Builder

	header := fmt.Sprintf("Goroutines: %d (at %s)", s.Count, s.Timestamp.Format("15:04:05"))
	b.WriteString(color.Wrap(header, colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	// group by state
	states := make(map[string]int)
	for _, g := range s.Goroutines {
		states[g.State]++
	}

	type stateCount struct {
		state string
		count int
	}
	var sorted []stateCount
	for s, c := range states {
		sorted = append(sorted, stateCount{s, c})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	for _, sc := range sorted {
		b.WriteString(fmt.Sprintf("  %-20s %d\n",
			color.Wrap(sc.state, colorize, color.Yellow),
			sc.count,
		))
	}

	b.WriteByte('\n')

	// list goroutines
	for _, g := range s.Goroutines {
		b.WriteString(fmt.Sprintf("  %s #%d [%s]\n",
			color.Wrap(g.Function, colorize, color.Green),
			g.ID,
			color.Wrap(g.State, colorize, color.Gray),
		))
	}

	return b.String()
}
