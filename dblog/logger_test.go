package dblog

import (
	"testing"
	"time"
)

func TestLoggerRecord(t *testing.T) {
	l := New()

	l.Record("SELECT * FROM users WHERE id = $1", []any{42}, 5*time.Millisecond, nil, 1)

	if l.Count() != 1 {
		t.Fatalf("count = %d, want 1", l.Count())
	}

	queries := l.Queries()
	q := queries[0]

	if q.Query != "SELECT * FROM users WHERE id = $1" {
		t.Errorf("query = %q", q.Query)
	}
	if q.Operation != "SELECT" {
		t.Errorf("operation = %q, want SELECT", q.Operation)
	}
	if q.Duration != 5*time.Millisecond {
		t.Errorf("duration = %v, want 5ms", q.Duration)
	}
	if q.Rows != 1 {
		t.Errorf("rows = %d, want 1", q.Rows)
	}
	if q.Error != "" {
		t.Errorf("error = %q, want empty", q.Error)
	}
	if q.Caller == "" {
		t.Error("expected non-empty caller")
	}
}

func TestLoggerRecordWithError(t *testing.T) {
	l := New()

	l.Record("INSERT INTO users (name) VALUES ($1)", []any{"alice"}, 2*time.Millisecond, &testError{"unique violation"}, 0)

	queries := l.Queries()
	q := queries[0]

	if q.Operation != "INSERT" {
		t.Errorf("operation = %q, want INSERT", q.Operation)
	}
	if q.Error != "unique violation" {
		t.Errorf("error = %q, want 'unique violation'", q.Error)
	}
}

func TestLoggerCallback(t *testing.T) {
	var called bool
	l := New(WithOnLog(func(ql QueryLog) {
		called = true
		if ql.Operation != "UPDATE" {
			t.Errorf("operation = %q, want UPDATE", ql.Operation)
		}
	}))

	l.Record("UPDATE users SET name = $1", []any{"bob"}, time.Millisecond, nil, 1)

	if !called {
		t.Error("callback not invoked")
	}
}

func TestLoggerLastQueries(t *testing.T) {
	l := New()
	for i := 0; i < 10; i++ {
		l.Record("SELECT 1", nil, time.Millisecond, nil, 1)
	}

	last := l.LastQueries(3)
	if len(last) != 3 {
		t.Errorf("LastQueries(3) len = %d, want 3", len(last))
	}
}

func TestLoggerClear(t *testing.T) {
	l := New()
	l.Record("SELECT 1", nil, time.Millisecond, nil, 1)
	l.Clear()
	if l.Count() != 0 {
		t.Errorf("count after clear = %d, want 0", l.Count())
	}
}

func TestExtractOperation(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"SELECT * FROM users", "SELECT"},
		{"insert into users (name) values ($1)", "INSERT"},
		{"  UPDATE users SET name = $1", "UPDATE"},
		{"DELETE FROM users WHERE id = $1", "DELETE"},
		{"CREATE TABLE users (id int)", "CREATE"},
		{"", "UNKNOWN"},
	}
	for _, tc := range tests {
		got := extractOperation(tc.query)
		if got != tc.want {
			t.Errorf("extractOperation(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestLoggerCapacity(t *testing.T) {
	l := New(WithCapacity(5))
	for i := 0; i < 10; i++ {
		l.Record("SELECT 1", nil, time.Millisecond, nil, 1)
	}
	if l.Count() != 5 {
		t.Errorf("count = %d, want 5 (capacity)", l.Count())
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
