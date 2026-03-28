package config

import (
	"strings"
	"testing"
)

type AppConfig struct {
	Host     string `json:"host" env:"APP_HOST"`
	Port     int    `json:"port"`
	Debug    bool
	DBUrl    string `devtool:"redact" env:"DATABASE_URL"`
	APIKey   string `devtool:"redact"`
	Features []string
}

type DBConfig struct {
	Driver  string
	MaxConn int
}

func TestRegisterAndSnapshot(t *testing.T) {
	v := New()
	cfg := AppConfig{
		Host:     "localhost",
		Port:     8080,
		Debug:    true,
		DBUrl:    "postgres://user:pass@localhost/db",
		APIKey:   "secret-key-123",
		Features: []string{"auth", "cache"},
	}

	v.Register("app", cfg)

	snaps := v.Snapshot()
	if len(snaps) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(snaps))
	}

	snap := snaps[0]
	if snap.Name != "app" {
		t.Errorf("name = %q, want app", snap.Name)
	}

	// check entries
	entryMap := make(map[string]Entry)
	for _, e := range snap.Entries {
		entryMap[e.Key] = e
	}

	if e, ok := entryMap["Host"]; !ok {
		t.Error("missing Host entry")
	} else {
		if e.Value != "localhost" {
			t.Errorf("Host value = %q", e.Value)
		}
		if e.Source != "env:APP_HOST" {
			t.Errorf("Host source = %q, want env:APP_HOST", e.Source)
		}
	}

	if e, ok := entryMap["Port"]; !ok {
		t.Error("missing Port entry")
	} else {
		if e.Value != "8080" {
			t.Errorf("Port value = %q", e.Value)
		}
		if e.Source != "json:port" {
			t.Errorf("Port source = %q, want json:port", e.Source)
		}
	}

	if e, ok := entryMap["DBUrl"]; !ok {
		t.Error("missing DBUrl entry")
	} else {
		if !e.Redacted {
			t.Error("DBUrl should be redacted")
		}
		if e.Value != "********" {
			t.Errorf("redacted value = %q, want ********", e.Value)
		}
	}

	if e, ok := entryMap["APIKey"]; !ok {
		t.Error("missing APIKey entry")
	} else {
		if !e.Redacted {
			t.Error("APIKey should be redacted")
		}
	}

	if e, ok := entryMap["Features"]; !ok {
		t.Error("missing Features entry")
	} else {
		if !strings.Contains(e.Value, "auth") {
			t.Errorf("Features value = %q, expected auth", e.Value)
		}
	}
}

func TestGet(t *testing.T) {
	v := New()
	v.Register("db", DBConfig{Driver: "postgres", MaxConn: 25})

	snap, ok := v.Get("db")
	if !ok {
		t.Fatal("expected to find db config")
	}
	if snap.Name != "db" {
		t.Errorf("name = %q", snap.Name)
	}

	_, ok = v.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestUnregister(t *testing.T) {
	v := New()
	v.Register("app", AppConfig{})
	v.Unregister("app")

	if len(v.Names()) != 0 {
		t.Errorf("names after unregister: %v", v.Names())
	}
}

func TestNames(t *testing.T) {
	v := New()
	v.Register("app", AppConfig{})
	v.Register("db", DBConfig{})

	names := v.Names()
	if len(names) != 2 {
		t.Errorf("names count = %d, want 2", len(names))
	}
}

func TestCustomSources(t *testing.T) {
	v := New()
	cfg := DBConfig{Driver: "postgres", MaxConn: 25}
	v.Register("db", cfg, map[string]string{
		"Driver":  "flag:--db-driver",
		"MaxConn": "env:DB_MAX_CONN",
	})

	snap, _ := v.Get("db")
	entryMap := make(map[string]Entry)
	for _, e := range snap.Entries {
		entryMap[e.Key] = e
	}

	if e := entryMap["Driver"]; e.Source != "flag:--db-driver" {
		t.Errorf("Driver source = %q", e.Source)
	}
	if e := entryMap["MaxConn"]; e.Source != "env:DB_MAX_CONN" {
		t.Errorf("MaxConn source = %q", e.Source)
	}
}

func TestFormatSnapshot(t *testing.T) {
	v := New()
	v.Register("app", AppConfig{
		Host:  "localhost",
		Port:  8080,
		DBUrl: "secret",
	})

	snap, _ := v.Get("app")
	output := FormatSnapshot(snap, false)

	if !strings.Contains(output, "app") {
		t.Errorf("expected 'app' in output: %s", output)
	}
	if !strings.Contains(output, "localhost") {
		t.Errorf("expected 'localhost' in output: %s", output)
	}
	if !strings.Contains(output, "********") {
		t.Errorf("expected redacted value in output: %s", output)
	}
}

func TestNilPointerConfig(t *testing.T) {
	v := New()
	var cfg *AppConfig
	v.Register("nil", cfg)

	snap, ok := v.Get("nil")
	if !ok {
		t.Fatal("expected to find nil config")
	}
	if len(snap.Entries) != 0 {
		t.Errorf("expected 0 entries for nil config, got %d", len(snap.Entries))
	}
}
