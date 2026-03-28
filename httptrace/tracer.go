package httptrace

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// RequestTrace captures timing details of an outgoing HTTP request.
type RequestTrace struct {
	ID               string        `json:"id"`
	Timestamp        time.Time     `json:"timestamp"`
	Method           string        `json:"method"`
	URL              string        `json:"url"`
	StatusCode       int           `json:"status_code"`
	Duration         time.Duration `json:"duration"`
	DNSLookup        time.Duration `json:"dns_lookup"`
	TCPConnect       time.Duration `json:"tcp_connect"`
	TLSHandshake     time.Duration `json:"tls_handshake"`
	ServerProcessing time.Duration `json:"server_processing"`
	ContentTransfer  time.Duration `json:"content_transfer"`
	Error            string        `json:"error,omitempty"`
	RequestSize      int64         `json:"request_size"`
	ResponseSize     int64         `json:"response_size"`
}

// Tracer instruments outgoing HTTP requests.
type Tracer struct {
	store   *ringbuf.Buffer[RequestTrace]
	onTrace func(RequestTrace)
	mu      sync.Mutex
	counter uint64
}

// Option configures the Tracer.
type Option func(*Tracer)

// WithCapacity sets the ring buffer capacity (default 200).
func WithCapacity(n int) Option {
	return func(t *Tracer) { t.store = ringbuf.New[RequestTrace](n) }
}

// WithOnTrace sets a callback invoked for each traced request.
func WithOnTrace(fn func(RequestTrace)) Option {
	return func(t *Tracer) { t.onTrace = fn }
}

// New creates a Tracer.
func New(opts ...Option) *Tracer {
	t := &Tracer{
		store: ringbuf.New[RequestTrace](200),
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// WrapClient returns a new *http.Client whose Transport is instrumented.
func (t *Tracer) WrapClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return &http.Client{
		Transport:     t.Transport(base),
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       client.Timeout,
	}
}

// Transport returns an http.RoundTripper that traces requests.
func (t *Tracer) Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &tracingTransport{tracer: t, base: base}
}

// Traces returns all captured traces.
func (t *Tracer) Traces() []RequestTrace {
	return t.store.All()
}

// LastTraces returns the n most recent traces.
func (t *Tracer) LastTraces(n int) []RequestTrace {
	return t.store.Last(n)
}

// Count returns total traced requests.
func (t *Tracer) Count() int {
	return t.store.Len()
}

// Clear removes all stored traces.
func (t *Tracer) Clear() {
	t.store.Clear()
}

func (t *Tracer) record(rt RequestTrace) {
	t.mu.Lock()
	t.counter++
	rt.ID = fmt.Sprintf("ht%08x", t.counter)
	t.mu.Unlock()

	t.store.Push(rt)

	if t.onTrace != nil {
		t.onTrace(rt)
	}
}

type tracingTransport struct {
	tracer *Tracer
	base   http.RoundTripper
}

func (tt *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		dnsStart, dnsEnd         time.Time
		connectStart, connectEnd time.Time
		tlsStart, tlsEnd         time.Time
		gotFirstByte             time.Time
		requestStart             = time.Now()
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsEnd = time.Now()
		},
		ConnectStart: func(network, addr string) {
			connectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			connectEnd = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			tlsEnd = time.Now()
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	ctx := httptrace.WithClientTrace(req.Context(), trace)
	req = req.WithContext(ctx)

	resp, err := tt.base.RoundTrip(req)
	requestEnd := time.Now()

	rt := RequestTrace{
		Timestamp: requestStart,
		Method:    req.Method,
		URL:       req.URL.String(),
		Duration:  requestEnd.Sub(requestStart),
	}

	if req.ContentLength > 0 {
		rt.RequestSize = req.ContentLength
	}

	// Compute timing phases
	if !dnsStart.IsZero() && !dnsEnd.IsZero() {
		rt.DNSLookup = dnsEnd.Sub(dnsStart)
	}
	if !connectStart.IsZero() && !connectEnd.IsZero() {
		rt.TCPConnect = connectEnd.Sub(connectStart)
	}
	if !tlsStart.IsZero() && !tlsEnd.IsZero() {
		rt.TLSHandshake = tlsEnd.Sub(tlsStart)
	}
	if !gotFirstByte.IsZero() {
		// Server processing = time from connection established to first byte
		if !connectEnd.IsZero() {
			rt.ServerProcessing = gotFirstByte.Sub(connectEnd)
		} else {
			rt.ServerProcessing = gotFirstByte.Sub(requestStart)
		}
		rt.ContentTransfer = requestEnd.Sub(gotFirstByte)
	}

	if err != nil {
		rt.Error = err.Error()
	} else {
		rt.StatusCode = resp.StatusCode
		rt.ResponseSize = resp.ContentLength
	}

	tt.tracer.record(rt)

	return resp, err
}

// contextKey is unexported to avoid collisions.
type contextKey struct{}

// WithTracing returns a context that enables tracing on a per-request basis.
func WithTracing(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, true)
}
