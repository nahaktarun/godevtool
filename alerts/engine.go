package alerts

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// Severity levels for alerts.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// AlertState represents the current state of an alert.
type AlertState string

const (
	StateOK       AlertState = "ok"
	StateFiring   AlertState = "firing"
	StateResolved AlertState = "resolved"
)

// Alert represents an alert event.
type Alert struct {
	ID        string     `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	RuleName  string     `json:"rule_name"`
	Severity  Severity   `json:"severity"`
	State     AlertState `json:"state"`
	Message   string     `json:"message"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
}

// Rule defines an alert condition.
type Rule struct {
	Name      string
	Severity  Severity
	Condition func() (value float64, firing bool)
	Message   string
	Threshold float64
	Cooldown  time.Duration
}

// RuleInfo is the JSON-serializable view of a Rule.
type RuleInfo struct {
	Name      string   `json:"name"`
	Severity  Severity `json:"severity"`
	Message   string   `json:"message"`
	Threshold float64  `json:"threshold"`
	Cooldown  string   `json:"cooldown"`
}

type ruleState struct {
	rule     Rule
	firing   bool
	lastFire time.Time
}

// Engine evaluates alert rules periodically.
type Engine struct {
	mu       sync.RWMutex
	rules    []ruleState
	history  *ringbuf.Buffer[Alert]
	active   map[string]*Alert
	onAlert  func(Alert)
	interval time.Duration
	stopCh   chan struct{}
	running  bool
	counter  uint64
}

// Option configures the Engine.
type Option func(*Engine)

// WithCheckInterval sets how often rules are evaluated (default 10s).
func WithCheckInterval(d time.Duration) Option {
	return func(e *Engine) { e.interval = d }
}

// WithOnAlert sets a callback invoked when an alert fires or resolves.
func WithOnAlert(fn func(Alert)) Option {
	return func(e *Engine) { e.onAlert = fn }
}

// WithCapacity sets the history ring buffer capacity (default 200).
func WithCapacity(n int) Option {
	return func(e *Engine) { e.history = ringbuf.New[Alert](n) }
}

// New creates an Engine.
func New(opts ...Option) *Engine {
	e := &Engine{
		history:  ringbuf.New[Alert](200),
		active:   make(map[string]*Alert),
		interval: 10 * time.Second,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// AddRule registers an alert rule.
func (e *Engine) AddRule(rule Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if rule.Cooldown == 0 {
		rule.Cooldown = time.Minute
	}
	e.rules = append(e.rules, ruleState{rule: rule})
}

// Start begins periodic rule evaluation.
func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return
	}
	e.running = true
	e.stopCh = make(chan struct{})
	go e.run()
}

// Stop halts the engine.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.running = false
	close(e.stopCh)
}

func (e *Engine) run() {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	e.evaluate() // initial check

	for {
		select {
		case <-ticker.C:
			e.evaluate()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) evaluate() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	for i := range e.rules {
		rs := &e.rules[i]
		value, firing := rs.rule.Condition()

		if firing && !rs.firing {
			// transition: ok -> firing
			if now.Sub(rs.lastFire) < rs.rule.Cooldown {
				continue // still in cooldown
			}
			rs.firing = true
			rs.lastFire = now

			e.counter++
			alert := Alert{
				ID:        fmt.Sprintf("alert%08x", e.counter),
				Timestamp: now,
				RuleName:  rs.rule.Name,
				Severity:  rs.rule.Severity,
				State:     StateFiring,
				Message:   rs.rule.Message,
				Value:     value,
				Threshold: rs.rule.Threshold,
			}
			e.history.Push(alert)
			e.active[rs.rule.Name] = &alert

			if e.onAlert != nil {
				e.onAlert(alert)
			}
		} else if !firing && rs.firing {
			// transition: firing -> resolved
			rs.firing = false

			e.counter++
			alert := Alert{
				ID:        fmt.Sprintf("alert%08x", e.counter),
				Timestamp: now,
				RuleName:  rs.rule.Name,
				Severity:  rs.rule.Severity,
				State:     StateResolved,
				Message:   rs.rule.Message + " (resolved)",
				Value:     value,
				Threshold: rs.rule.Threshold,
			}
			e.history.Push(alert)
			delete(e.active, rs.rule.Name)

			if e.onAlert != nil {
				e.onAlert(alert)
			}
		}
	}
}

// Alerts returns all alert history.
func (e *Engine) Alerts() []Alert {
	return e.history.All()
}

// ActiveAlerts returns currently firing alerts.
func (e *Engine) ActiveAlerts() []Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]Alert, 0, len(e.active))
	for _, a := range e.active {
		result = append(result, *a)
	}
	return result
}

// ActiveCount returns the number of currently firing alerts.
func (e *Engine) ActiveCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.active)
}

// Rules returns all registered rules as serializable info.
func (e *Engine) Rules() []RuleInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]RuleInfo, len(e.rules))
	for i, rs := range e.rules {
		result[i] = RuleInfo{
			Name:      rs.rule.Name,
			Severity:  rs.rule.Severity,
			Message:   rs.rule.Message,
			Threshold: rs.rule.Threshold,
			Cooldown:  rs.rule.Cooldown.String(),
		}
	}
	return result
}

// --- Built-in rule constructors ---

// GoroutineCountRule alerts when goroutine count exceeds threshold.
func GoroutineCountRule(threshold int, severity Severity) Rule {
	return Rule{
		Name:      "goroutine_count",
		Severity:  severity,
		Threshold: float64(threshold),
		Condition: func() (float64, bool) {
			count := float64(runtime.NumGoroutine())
			return count, count > float64(threshold)
		},
		Message:  fmt.Sprintf("goroutine count exceeds %d", threshold),
		Cooldown: time.Minute,
	}
}

// HeapAllocRule alerts when heap allocation exceeds threshold bytes.
func HeapAllocRule(thresholdBytes uint64, severity Severity) Rule {
	return Rule{
		Name:      "heap_alloc",
		Severity:  severity,
		Threshold: float64(thresholdBytes),
		Condition: func() (float64, bool) {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.HeapAlloc), m.HeapAlloc > thresholdBytes
		},
		Message:  fmt.Sprintf("heap alloc exceeds %d bytes", thresholdBytes),
		Cooldown: time.Minute,
	}
}

// CustomRule creates a rule with a custom condition function.
func CustomRule(name string, severity Severity, threshold float64, message string, condition func() (float64, bool)) Rule {
	return Rule{
		Name:      name,
		Severity:  severity,
		Threshold: threshold,
		Condition: condition,
		Message:   message,
		Cooldown:  time.Minute,
	}
}

// FormatAlerts returns a human-readable string.
func FormatAlerts(alerts []Alert, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("Alerts", colorize, color.Red, color.Bold))
	b.WriteByte('\n')

	if len(alerts) == 0 {
		b.WriteString("  No alerts.\n")
		return b.String()
	}

	for _, a := range alerts {
		sevColor := color.Green
		switch a.Severity {
		case SeverityWarning:
			sevColor = color.Yellow
		case SeverityCritical:
			sevColor = color.Red
		}

		stateStr := string(a.State)
		if a.State == StateFiring {
			stateStr = color.Wrap("FIRING", colorize, color.Red, color.Bold)
		} else if a.State == StateResolved {
			stateStr = color.Wrap("RESOLVED", colorize, color.Green)
		}

		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s  val=%.0f\n",
			color.Wrap(a.Timestamp.Format("15:04:05"), colorize, color.Gray),
			color.Wrap(string(a.Severity), colorize, sevColor),
			stateStr,
			a.Message,
			a.Value))
	}

	return b.String()
}
