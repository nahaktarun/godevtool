package grpcmon

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/color"
	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// CallLog records a single gRPC call.
type CallLog struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Method    string        `json:"method"`
	Service   string        `json:"service"`
	Type      string        `json:"type"` // "unary" or "stream"
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	IsServer  bool          `json:"is_server"`
}

// Monitor captures gRPC call metrics.
type Monitor struct {
	store   *ringbuf.Buffer[CallLog]
	onCall  func(CallLog)
	mu      sync.Mutex
	counter uint64
}

// Option configures the Monitor.
type Option func(*Monitor)

// WithCapacity sets the ring buffer capacity (default 200).
func WithCapacity(n int) Option {
	return func(m *Monitor) { m.store = ringbuf.New[CallLog](n) }
}

// WithOnCall sets a callback invoked for each logged call.
func WithOnCall(fn func(CallLog)) Option {
	return func(m *Monitor) { m.onCall = fn }
}

// New creates a Monitor.
func New(opts ...Option) *Monitor {
	m := &Monitor{
		store: ringbuf.New[CallLog](200),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Record manually logs a gRPC call. Use this if the interceptor approach
// doesn't fit your setup.
func (m *Monitor) Record(method string, duration time.Duration, err error, isServer bool, callType string) {
	m.mu.Lock()
	m.counter++
	id := fmt.Sprintf("grpc%08x", m.counter)
	m.mu.Unlock()

	service, _ := splitMethod(method)

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	entry := CallLog{
		ID:        id,
		Timestamp: time.Now(),
		Method:    method,
		Service:   service,
		Type:      callType,
		Duration:  duration,
		Error:     errStr,
		IsServer:  isServer,
	}

	m.store.Push(entry)

	if m.onCall != nil {
		m.onCall(entry)
	}
}

// UnaryServerInterceptor returns a function compatible with grpc.UnaryServerInterceptor.
// Cast with: interceptor.(grpc.UnaryServerInterceptor)
//
// Signature: func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error)
func (m *Monitor) UnaryServerInterceptor() any {
	return func(ctx context.Context, req any, info any, handler func(context.Context, any) (any, error)) (any, error) {
		method := extractMethodFromInfo(info)
		start := time.Now()
		resp, err := handler(ctx, req)
		m.Record(method, time.Since(start), err, true, "unary")
		return resp, err
	}
}

// StreamServerInterceptor returns a function compatible with grpc.StreamServerInterceptor.
func (m *Monitor) StreamServerInterceptor() any {
	return func(srv any, ss any, info any, handler func(any, any) error) error {
		method := extractMethodFromInfo(info)
		start := time.Now()
		err := handler(srv, ss)
		m.Record(method, time.Since(start), err, true, "stream")
		return err
	}
}

// UnaryClientInterceptor returns a function compatible with grpc.UnaryClientInterceptor.
func (m *Monitor) UnaryClientInterceptor() any {
	return func(ctx context.Context, method string, req, reply any, cc any, invoker func(context.Context, string, any, any, any, ...any) error, opts ...any) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		m.Record(method, time.Since(start), err, false, "unary")
		return err
	}
}

// Calls returns all logged calls.
func (m *Monitor) Calls() []CallLog {
	return m.store.All()
}

// LastCalls returns the n most recent calls.
func (m *Monitor) LastCalls(n int) []CallLog {
	return m.store.Last(n)
}

// Count returns total logged calls.
func (m *Monitor) Count() int {
	return m.store.Len()
}

// Clear removes all stored calls.
func (m *Monitor) Clear() {
	m.store.Clear()
}

func splitMethod(fullMethod string) (service, method string) {
	// gRPC methods are formatted as "/package.Service/Method"
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	parts := strings.SplitN(fullMethod, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return fullMethod, ""
}

func extractMethodFromInfo(info any) string {
	// Use reflection-free approach: info is expected to have a FullMethod field
	// but since we can't import grpc, we use fmt to extract it
	s := fmt.Sprintf("%v", info)
	// grpc.UnaryServerInfo prints as &{FullMethod:/package.Service/Method ...}
	if idx := strings.Index(s, "/"); idx >= 0 {
		end := strings.IndexAny(s[idx:], " }")
		if end > 0 {
			return s[idx : idx+end]
		}
		return s[idx:]
	}
	return "unknown"
}

// FormatCalls returns a human-readable string.
func FormatCalls(calls []CallLog, colorize bool) string {
	var b strings.Builder

	b.WriteString(color.Wrap("gRPC Calls", colorize, color.Cyan, color.Bold))
	b.WriteByte('\n')

	if len(calls) == 0 {
		b.WriteString("  No gRPC calls recorded.\n")
		return b.String()
	}

	for _, c := range calls {
		side := "client"
		if c.IsServer {
			side = "server"
		}
		errStr := ""
		if c.Error != "" {
			errStr = color.Wrap(" err="+c.Error, colorize, color.Red)
		}
		b.WriteString(fmt.Sprintf("  %s  [%s] %s  %s  %s%s\n",
			color.Wrap(c.Timestamp.Format("15:04:05"), colorize, color.Gray),
			side,
			color.Wrap(c.Type, colorize, color.Yellow),
			color.Wrap(c.Method, colorize, color.Green),
			c.Duration,
			errStr))
	}

	return b.String()
}
