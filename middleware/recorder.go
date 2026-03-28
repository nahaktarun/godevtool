package middleware

import (
	"bytes"
	"net/http"
)

// responseRecorder wraps http.ResponseWriter to capture status code and body.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	maxBody    int
	written    bool
}

func newResponseRecorder(w http.ResponseWriter, maxBody int) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		maxBody:        maxBody,
	}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.written = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.written = true
	}
	// capture up to maxBody bytes
	if r.body.Len() < r.maxBody {
		remaining := r.maxBody - r.body.Len()
		if len(b) <= remaining {
			r.body.Write(b)
		} else {
			r.body.Write(b[:remaining])
		}
	}
	return r.ResponseWriter.Write(b)
}
