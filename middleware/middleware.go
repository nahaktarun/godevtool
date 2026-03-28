package middleware

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/tarunnahak/godevtool/internal/ringbuf"
)

// RequestLog captures a single request/response cycle.
type RequestLog struct {
	ID              string
	Timestamp       time.Time
	Method          string
	Path            string
	Query           string
	StatusCode      int
	Duration        time.Duration
	RequestHeaders  http.Header
	ResponseHeaders http.Header
	RequestBody     []byte
	ResponseBody    []byte
	ClientIP        string
	ContentLength   int64
}

// Inspector captures HTTP request/response data.
type Inspector struct {
	store   *ringbuf.Buffer[RequestLog]
	onLog   func(RequestLog)
	maxBody int
	mu      sync.Mutex
	counter uint64
}

// Option configures the Inspector.
type Option func(*Inspector)

// WithCapacity sets the ring buffer capacity (default 200).
func WithCapacity(n int) Option {
	return func(ins *Inspector) {
		ins.store = ringbuf.New[RequestLog](n)
	}
}

// WithMaxBodyCapture sets the max bytes captured for request/response bodies (default 4KB).
func WithMaxBodyCapture(n int) Option {
	return func(ins *Inspector) { ins.maxBody = n }
}

// WithOnLog sets a callback invoked for each captured request.
func WithOnLog(fn func(RequestLog)) Option {
	return func(ins *Inspector) { ins.onLog = fn }
}

// New creates an Inspector.
func New(opts ...Option) *Inspector {
	ins := &Inspector{
		store:   ringbuf.New[RequestLog](200),
		maxBody: 4096,
	}
	for _, o := range opts {
		o(ins)
	}
	return ins
}

// Handler returns an http.Handler middleware wrapper.
func (ins *Inspector) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ins.handle(w, r, next)
	})
}

// HandlerFunc returns the middleware as a function compatible with mux wrappers.
func (ins *Inspector) HandlerFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ins.handle(w, r, next)
	}
}

func (ins *Inspector) handle(w http.ResponseWriter, r *http.Request, next http.Handler) {
	start := time.Now()

	// capture request body
	var reqBody []byte
	if r.Body != nil {
		reqBody, _ = io.ReadAll(io.LimitReader(r.Body, int64(ins.maxBody)))
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	// wrap response writer
	rec := newResponseRecorder(w, ins.maxBody)
	next.ServeHTTP(rec, r)

	duration := time.Since(start)

	ins.mu.Lock()
	ins.counter++
	id := ins.counter
	ins.mu.Unlock()

	entry := RequestLog{
		ID:              formatID(id),
		Timestamp:       start,
		Method:          r.Method,
		Path:            r.URL.Path,
		Query:           r.URL.RawQuery,
		StatusCode:      rec.statusCode,
		Duration:        duration,
		RequestHeaders:  r.Header.Clone(),
		ResponseHeaders: rec.Header().Clone(),
		RequestBody:     reqBody,
		ResponseBody:    rec.body.Bytes(),
		ClientIP:        r.RemoteAddr,
		ContentLength:   r.ContentLength,
	}

	ins.store.Push(entry)

	if ins.onLog != nil {
		ins.onLog(entry)
	}
}

// Requests returns all captured request logs.
func (ins *Inspector) Requests() []RequestLog {
	return ins.store.All()
}

// LastRequests returns the n most recent request logs.
func (ins *Inspector) LastRequests(n int) []RequestLog {
	return ins.store.Last(n)
}

// Clear empties the request log.
func (ins *Inspector) Clear() {
	ins.store.Clear()
}

// Count returns the total number of stored requests.
func (ins *Inspector) Count() int {
	return ins.store.Len()
}

// SetOnLog replaces the callback invoked for each captured request.
func (ins *Inspector) SetOnLog(fn func(RequestLog)) {
	ins.mu.Lock()
	defer ins.mu.Unlock()
	ins.onLog = fn
}

func formatID(n uint64) string {
	const chars = "0123456789abcdef"
	buf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		buf[i] = chars[n&0xf]
		n >>= 4
	}
	return string(buf)
}
