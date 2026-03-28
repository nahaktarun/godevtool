package dblog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// QueryLog records a single database query.
type QueryLog struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Query     string        `json:"query"`
	Args      []any         `json:"args,omitempty"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	Rows      int64         `json:"rows"`
	Caller    string        `json:"caller"`
	Operation string        `json:"operation"` // SELECT, INSERT, UPDATE, DELETE, etc.
}

// Logger captures and stores database query logs.
type Logger struct {
	store   *ringbuf.Buffer[QueryLog]
	onLog   func(QueryLog)
	mu      sync.Mutex
	counter uint64
}

// Option configures the Logger.
type Option func(*Logger)

// WithCapacity sets the ring buffer capacity (default 500).
func WithCapacity(n int) Option {
	return func(l *Logger) {
		l.store = ringbuf.New[QueryLog](n)
	}
}

// WithOnLog sets a callback invoked for each captured query.
func WithOnLog(fn func(QueryLog)) Option {
	return func(l *Logger) { l.onLog = fn }
}

// New creates a query Logger.
func New(opts ...Option) *Logger {
	l := &Logger{
		store: ringbuf.New[QueryLog](500),
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Record manually logs a query execution. Use this when wrapping queries yourself.
func (l *Logger) Record(query string, args []any, duration time.Duration, err error, rows int64) {
	l.mu.Lock()
	l.counter++
	id := l.counter
	l.mu.Unlock()

	errStr := ""
	if err != nil && err != sql.ErrNoRows {
		errStr = err.Error()
	}

	entry := QueryLog{
		ID:        fmt.Sprintf("q%08x", id),
		Timestamp: time.Now(),
		Query:     query,
		Args:      args,
		Duration:  duration,
		Error:     errStr,
		Rows:      rows,
		Caller:    getCaller(3),
		Operation: extractOperation(query),
	}

	l.store.Push(entry)

	if l.onLog != nil {
		l.onLog(entry)
	}
}

// Queries returns all captured query logs.
func (l *Logger) Queries() []QueryLog {
	return l.store.All()
}

// LastQueries returns the n most recent query logs.
func (l *Logger) LastQueries(n int) []QueryLog {
	return l.store.Last(n)
}

// Clear removes all stored queries.
func (l *Logger) Clear() {
	l.store.Clear()
}

// Count returns the number of stored queries.
func (l *Logger) Count() int {
	return l.store.Len()
}

// --- Wrapped DB ---

// DB wraps a *sql.DB to automatically log all queries.
type DB struct {
	*sql.DB
	logger *Logger
}

// WrapDB wraps an existing *sql.DB to log all queries.
func WrapDB(db *sql.DB, logger *Logger) *DB {
	return &DB{DB: db, logger: logger}
}

// QueryContext executes a query and logs it.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.DB.QueryContext(ctx, query, args...)
	d.logger.Record(query, args, time.Since(start), err, -1)
	return rows, err
}

// Query executes a query and logs it.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.QueryContext(context.Background(), query, args...)
}

// ExecContext executes a statement and logs it.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := d.DB.ExecContext(ctx, query, args...)
	var rowsAffected int64 = -1
	if err == nil && result != nil {
		rowsAffected, _ = result.RowsAffected()
	}
	d.logger.Record(query, args, time.Since(start), err, rowsAffected)
	return result, err
}

// Exec executes a statement and logs it.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.ExecContext(context.Background(), query, args...)
}

// QueryRowContext executes a query returning a single row and logs it.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.DB.QueryRowContext(ctx, query, args...)
	d.logger.Record(query, args, time.Since(start), nil, 1)
	return row
}

// QueryRow executes a query returning a single row and logs it.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.QueryRowContext(context.Background(), query, args...)
}

// --- Wrapped Tx ---

// Tx wraps a *sql.Tx to automatically log all queries.
type Tx struct {
	*sql.Tx
	logger *Logger
}

// BeginTx starts a transaction and returns a wrapped Tx.
func (d *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := d.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, logger: d.logger}, nil
}

// Begin starts a transaction and returns a wrapped Tx.
func (d *DB) Begin() (*Tx, error) {
	return d.BeginTx(context.Background(), nil)
}

// ExecContext executes a statement within the transaction and logs it.
func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := t.Tx.ExecContext(ctx, query, args...)
	var rowsAffected int64 = -1
	if err == nil && result != nil {
		rowsAffected, _ = result.RowsAffected()
	}
	t.logger.Record(query, args, time.Since(start), err, rowsAffected)
	return result, err
}

// QueryContext executes a query within the transaction and logs it.
func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := t.Tx.QueryContext(ctx, query, args...)
	t.logger.Record(query, args, time.Since(start), err, -1)
	return rows, err
}

// --- Helpers ---

func getCaller(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	funcName := "unknown"
	if fn != nil {
		parts := strings.Split(fn.Name(), "/")
		funcName = parts[len(parts)-1]
	}
	// shorten file path
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		file = strings.Join(parts[len(parts)-2:], "/")
	}
	return fmt.Sprintf("%s (%s:%d)", funcName, file, line)
}

func extractOperation(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "UNKNOWN"
	}
	// find first word
	end := strings.IndexAny(q, " \t\n\r")
	if end == -1 {
		end = len(q)
	}
	return strings.ToUpper(q[:end])
}

// Ensure DB satisfies the driver.Driver interface pattern is not needed —
// we wrap at the sql.DB level, not driver level, for maximum compatibility.
var _ driver.Driver = (*wrappedDriver)(nil)

// wrappedDriver is unused but kept to show the interface is understood.
// The actual wrapping happens at the sql.DB/Tx level above.
type wrappedDriver struct{}

func (wrappedDriver) Open(string) (driver.Conn, error) { return nil, fmt.Errorf("not implemented") }
