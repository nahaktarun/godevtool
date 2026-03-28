package alerts

import (
	"strings"
	"testing"
	"time"
)

func TestAddRuleAndEvaluate(t *testing.T) {
	var fired bool
	e := New(
		WithCheckInterval(50*time.Millisecond),
		WithOnAlert(func(a Alert) {
			fired = true
		}),
	)

	e.AddRule(Rule{
		Name:     "test_rule",
		Severity: SeverityWarning,
		Condition: func() (float64, bool) {
			return 100, true // always firing
		},
		Message:   "test alert",
		Threshold: 50,
		Cooldown:  time.Second,
	})

	e.Start()
	time.Sleep(200 * time.Millisecond)
	e.Stop()

	if !fired {
		t.Error("alert callback not invoked")
	}

	active := e.ActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("active alerts = %d, want 1", len(active))
	}
	if active[0].RuleName != "test_rule" {
		t.Errorf("rule name = %q", active[0].RuleName)
	}
	if active[0].State != StateFiring {
		t.Errorf("state = %q, want firing", active[0].State)
	}
}

func TestAlertResolution(t *testing.T) {
	firing := true
	var states []AlertState

	e := New(
		WithCheckInterval(50*time.Millisecond),
		WithOnAlert(func(a Alert) {
			states = append(states, a.State)
		}),
	)

	e.AddRule(Rule{
		Name:     "flip_rule",
		Severity: SeverityInfo,
		Condition: func() (float64, bool) {
			return 1, firing
		},
		Message:  "flipping alert",
		Cooldown: 0, // no cooldown for test
	})

	e.Start()
	time.Sleep(100 * time.Millisecond)

	firing = false // resolve
	time.Sleep(150 * time.Millisecond)
	e.Stop()

	if len(states) < 2 {
		t.Fatalf("expected at least 2 state transitions, got %d", len(states))
	}
	if states[0] != StateFiring {
		t.Errorf("first state = %q, want firing", states[0])
	}
	if states[1] != StateResolved {
		t.Errorf("second state = %q, want resolved", states[1])
	}

	if e.ActiveCount() != 0 {
		t.Errorf("active count = %d after resolution", e.ActiveCount())
	}
}

func TestGoroutineCountRule(t *testing.T) {
	rule := GoroutineCountRule(1, SeverityWarning)
	value, firing := rule.Condition()

	// There should be at least 2 goroutines (main + test)
	if value < 2 {
		t.Errorf("goroutine count = %.0f, expected >= 2", value)
	}
	if !firing {
		t.Error("should be firing (threshold=1)")
	}

	rule2 := GoroutineCountRule(999999, SeverityWarning)
	_, firing2 := rule2.Condition()
	if firing2 {
		t.Error("should not fire with very high threshold")
	}
}

func TestHeapAllocRule(t *testing.T) {
	rule := HeapAllocRule(1, SeverityCritical)
	value, firing := rule.Condition()

	if value <= 0 {
		t.Error("heap alloc should be > 0")
	}
	if !firing {
		t.Error("should fire with threshold=1 byte")
	}
}

func TestCustomRule(t *testing.T) {
	rule := CustomRule("test", SeverityInfo, 10, "custom alert",
		func() (float64, bool) { return 20, true })

	if rule.Name != "test" {
		t.Errorf("name = %q", rule.Name)
	}
	v, f := rule.Condition()
	if v != 20 || !f {
		t.Error("custom condition failed")
	}
}

func TestRules(t *testing.T) {
	e := New()
	e.AddRule(GoroutineCountRule(100, SeverityWarning))
	e.AddRule(HeapAllocRule(1024*1024*1024, SeverityCritical))

	rules := e.Rules()
	if len(rules) != 2 {
		t.Errorf("rules count = %d", len(rules))
	}
}

func TestAlertHistory(t *testing.T) {
	e := New(WithCheckInterval(50 * time.Millisecond))

	e.AddRule(Rule{
		Name:     "always_fire",
		Severity: SeverityInfo,
		Condition: func() (float64, bool) {
			return 1, true
		},
		Message:  "test",
		Cooldown: 0,
	})

	e.Start()
	time.Sleep(100 * time.Millisecond)
	e.Stop()

	history := e.Alerts()
	if len(history) == 0 {
		t.Error("expected alert history")
	}
}

func TestFormatAlerts(t *testing.T) {
	alerts := []Alert{
		{Timestamp: time.Now(), RuleName: "test", Severity: SeverityWarning, State: StateFiring, Message: "test alert", Value: 50},
	}
	output := FormatAlerts(alerts, false)
	if !strings.Contains(output, "test alert") {
		t.Errorf("expected message in output: %s", output)
	}
}
