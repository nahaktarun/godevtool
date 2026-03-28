package cachemon

import (
	"strings"
	"testing"
)

func TestRegisterAndStats(t *testing.T) {
	m := New()
	r := m.Register("users")

	r.Hit()
	r.Hit()
	r.Miss()
	r.Set()
	r.SetSize(100)

	stats := m.Stats()
	if len(stats) != 1 {
		t.Fatalf("stats count = %d", len(stats))
	}

	s := stats[0]
	if s.Name != "users" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Hits != 2 {
		t.Errorf("hits = %d", s.Hits)
	}
	if s.Misses != 1 {
		t.Errorf("misses = %d", s.Misses)
	}
	if s.Sets != 1 {
		t.Errorf("sets = %d", s.Sets)
	}
	if s.Size != 100 {
		t.Errorf("size = %d", s.Size)
	}

	// 2 hits / 3 total = 0.666...
	if s.HitRate < 0.66 || s.HitRate > 0.67 {
		t.Errorf("hit rate = %f", s.HitRate)
	}
}

func TestEvictAndDelete(t *testing.T) {
	m := New()
	r := m.Register("cache1")

	r.Evict()
	r.Evict()
	r.Delete()

	s, ok := m.StatsFor("cache1")
	if !ok {
		t.Fatal("cache1 not found")
	}
	if s.Evictions != 2 {
		t.Errorf("evictions = %d", s.Evictions)
	}
	if s.Deletes != 1 {
		t.Errorf("deletes = %d", s.Deletes)
	}
}

func TestMultipleCaches(t *testing.T) {
	m := New()
	m.Register("cache1")
	m.Register("cache2")

	names := m.Names()
	if len(names) != 2 {
		t.Errorf("names = %d", len(names))
	}
	if m.Count() != 2 {
		t.Errorf("count = %d", m.Count())
	}
}

func TestStatsForNotFound(t *testing.T) {
	m := New()
	_, ok := m.StatsFor("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestHitRateZero(t *testing.T) {
	m := New()
	r := m.Register("empty")
	s := r.Stats()
	if s.HitRate != 0 {
		t.Errorf("hit rate = %f, want 0", s.HitRate)
	}
}

func TestCallback(t *testing.T) {
	var events []string
	m := New(WithOnEvent(func(name string, evt string) {
		events = append(events, name+":"+evt)
	}))

	r := m.Register("test")
	r.Hit()
	r.Miss()

	if len(events) != 2 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0] != "test:hit" {
		t.Errorf("event[0] = %q", events[0])
	}
}

func TestFormatStats(t *testing.T) {
	m := New()
	r := m.Register("users")
	r.Hit()
	r.Miss()

	output := FormatStats(m.Stats(), false)
	if !strings.Contains(output, "users") {
		t.Errorf("expected 'users' in output: %s", output)
	}
}
