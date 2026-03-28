package timer

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// Stats holds aggregate timing statistics for a single label.
type Stats struct {
	Label string
	Count int
	Total time.Duration
	Min   time.Duration
	Max   time.Duration
	Avg   time.Duration
	Last  time.Duration
}

// Report aggregates timing data for repeated operations.
type Report struct {
	mu      sync.Mutex
	entries map[string]*Stats
}

// NewReport creates an empty Report.
func NewReport() *Report {
	return &Report{
		entries: make(map[string]*Stats),
	}
}

// Record adds a timing entry. Thread-safe.
func (r *Report) Record(label string, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.entries[label]
	if !ok {
		s = &Stats{
			Label: label,
			Min:   d,
			Max:   d,
		}
		r.entries[label] = s
	}

	s.Count++
	s.Total += d
	s.Last = d
	s.Avg = s.Total / time.Duration(s.Count)

	if d < s.Min {
		s.Min = d
	}
	if d > s.Max {
		s.Max = d
	}
}

// Get returns stats for a label.
func (r *Report) Get(label string) (Stats, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.entries[label]
	if !ok {
		return Stats{}, false
	}
	return *s, true
}

// All returns stats for all labels, sorted by total time descending.
func (r *Report) All() []Stats {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]Stats, 0, len(r.entries))
	for _, s := range r.entries {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Total > result[j].Total
	})
	return result
}

// PrintTo writes a formatted table of all stats to w.
func (r *Report) PrintTo(w io.Writer) {
	stats := r.All()
	if len(stats) == 0 {
		fmt.Fprintln(w, "No timing data recorded.")
		return
	}

	// find max label width
	maxLabel := 5
	for _, s := range stats {
		if len(s.Label) > maxLabel {
			maxLabel = len(s.Label)
		}
	}

	header := fmt.Sprintf("%-*s  %6s  %12s  %12s  %12s  %12s  %12s\n",
		maxLabel, "LABEL", "COUNT", "TOTAL", "AVG", "MIN", "MAX", "LAST")
	fmt.Fprint(w, header)
	for i := 0; i < len(header)-1; i++ {
		fmt.Fprint(w, "─")
	}
	fmt.Fprintln(w)

	for _, s := range stats {
		fmt.Fprintf(w, "%-*s  %6d  %12s  %12s  %12s  %12s  %12s\n",
			maxLabel, s.Label, s.Count,
			s.Total.Round(time.Microsecond),
			s.Avg.Round(time.Microsecond),
			s.Min.Round(time.Microsecond),
			s.Max.Round(time.Microsecond),
			s.Last.Round(time.Microsecond),
		)
	}
}

// Reset clears all recorded data.
func (r *Report) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = make(map[string]*Stats)
}
