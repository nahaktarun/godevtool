package bench

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// Result holds benchmark statistics.
type Result struct {
	Label      string        `json:"label"`
	Iterations int           `json:"iterations"`
	TotalTime  time.Duration `json:"total_time"`
	AvgTime    time.Duration `json:"avg_time"`
	MinTime    time.Duration `json:"min_time"`
	MaxTime    time.Duration `json:"max_time"`
	P50        time.Duration `json:"p50"`
	P90        time.Duration `json:"p90"`
	P99        time.Duration `json:"p99"`
	OpsPerSec  float64       `json:"ops_per_sec"`
	Timestamp  time.Time     `json:"timestamp"`
}

// Runner executes benchmarks and stores results.
type Runner struct {
	store    *ringbuf.Buffer[Result]
	onResult func(Result)
}

// Option configures the Runner.
type Option func(*Runner)

// WithOnResult sets a callback invoked after each benchmark.
func WithOnResult(fn func(Result)) Option {
	return func(r *Runner) { r.onResult = fn }
}

// New creates a Runner.
func New(opts ...Option) *Runner {
	r := &Runner{
		store: ringbuf.New[Result](100),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Run executes fn n times and returns statistics.
func (r *Runner) Run(label string, n int, fn func()) Result {
	return r.RunWithSetup(label, n, nil, fn)
}

// RunWithSetup executes fn n times, calling setup before each iteration.
// Setup time is not measured.
func (r *Runner) RunWithSetup(label string, n int, setup func(), fn func()) Result {
	if n <= 0 {
		n = 1
	}

	durations := make([]time.Duration, n)
	totalStart := time.Now()

	for i := 0; i < n; i++ {
		if setup != nil {
			setup()
		}
		start := time.Now()
		fn()
		durations[i] = time.Since(start)
	}

	totalTime := time.Since(totalStart)

	// Sort for percentile computation
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}

	result := Result{
		Label:      label,
		Iterations: n,
		TotalTime:  totalTime,
		AvgTime:    sum / time.Duration(n),
		MinTime:    durations[0],
		MaxTime:    durations[n-1],
		P50:        percentile(durations, 0.50),
		P90:        percentile(durations, 0.90),
		P99:        percentile(durations, 0.99),
		Timestamp:  time.Now(),
	}

	if totalTime > 0 {
		result.OpsPerSec = float64(n) / totalTime.Seconds()
	}

	r.store.Push(result)

	if r.onResult != nil {
		r.onResult(result)
	}

	return result
}

// Results returns all past benchmark results.
func (r *Runner) Results() []Result {
	return r.store.All()
}

// LastResults returns the n most recent results.
func (r *Runner) LastResults(n int) []Result {
	return r.store.Last(n)
}

// Count returns the number of stored results.
func (r *Runner) Count() int {
	return r.store.Len()
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// FormatResult returns a human-readable string of a benchmark result.
func FormatResult(r Result, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap(r.Label, colorize, color.Cyan, color.Bold))
	b.WriteString(fmt.Sprintf("  (%d iterations)\n", r.Iterations))

	rows := []struct {
		label string
		value string
	}{
		{"Total", r.TotalTime.String()},
		{"Avg", r.AvgTime.String()},
		{"Min", r.MinTime.String()},
		{"Max", r.MaxTime.String()},
		{"P50", r.P50.String()},
		{"P90", r.P90.String()},
		{"P99", r.P99.String()},
		{"Ops/sec", fmt.Sprintf("%.0f", r.OpsPerSec)},
	}

	for _, row := range rows {
		b.WriteString(fmt.Sprintf("  %-8s %s\n",
			color.Wrap(row.label, colorize, color.Blue),
			row.value))
	}

	return b.String()
}

// FormatResults returns a table of all results.
func FormatResults(results []Result, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Benchmark Results", colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	if len(results) == 0 {
		b.WriteString("  No benchmarks run yet.\n")
		return b.String()
	}

	maxLabel := 5
	for _, r := range results {
		if len(r.Label) > maxLabel {
			maxLabel = len(r.Label)
		}
	}

	header := fmt.Sprintf("  %-*s  %6s  %12s  %12s  %12s  %12s  %10s\n",
		maxLabel, "LABEL", "N", "AVG", "P50", "P90", "P99", "OPS/SEC")
	b.WriteString(header)

	for _, r := range results {
		b.WriteString(fmt.Sprintf("  %-*s  %6d  %12s  %12s  %12s  %12s  %10.0f\n",
			maxLabel, r.Label, r.Iterations,
			r.AvgTime, r.P50, r.P90, r.P99, r.OpsPerSec))
	}

	return b.String()
}
