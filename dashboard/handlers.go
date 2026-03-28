package dashboard

import (
	"encoding/json"
	"fmt"
	stdlog "log"
	"net/http"
	"time"

	"github.com/tarunnahak/godevtool/bench"
	"github.com/tarunnahak/godevtool/cachemon"
	"github.com/tarunnahak/godevtool/config"
	"github.com/tarunnahak/godevtool/dblog"
	"github.com/tarunnahak/godevtool/deps"
	"github.com/tarunnahak/godevtool/environ"
	"github.com/tarunnahak/godevtool/errtrack"
	"github.com/tarunnahak/godevtool/goroutine"
	"github.com/tarunnahak/godevtool/httptrace"
	"github.com/tarunnahak/godevtool/log"
	"github.com/tarunnahak/godevtool/memstats"
	"github.com/tarunnahak/godevtool/middleware"
	"github.com/tarunnahak/godevtool/profiler"
	"github.com/tarunnahak/godevtool/ratelimit"
	"github.com/tarunnahak/godevtool/timeline"
	"github.com/tarunnahak/godevtool/timer"
)

// DataProviders bridges subsystems to the dashboard.
type DataProviders struct {
	Logger     *log.Logger
	Middleware *middleware.Inspector
	Goroutine  func() goroutine.Snapshot
	MemStats   func() memstats.Snapshot
	Timer      *timer.Report
	DBLogger   *dblog.Logger
	Timeline   *timeline.Timeline
	Config     *config.Viewer
	// Phase 5
	Environ    func() environ.Info
	Deps       func() deps.ScanResult
	ErrTracker *errtrack.Tracker
	Profiler   *profiler.Profiler
	// Phase 6
	HTTPTracer *httptrace.Tracer
	CacheMon   *cachemon.Monitor
	RateMon    *ratelimit.Monitor
	Bench      *bench.Runner
}

func (s *Server) registerAPIRoutes() {
	s.mux.HandleFunc("/api/logs", s.handleLogs)
	s.mux.HandleFunc("/api/requests", s.handleRequests)
	s.mux.HandleFunc("/api/goroutines", s.handleGoroutines)
	s.mux.HandleFunc("/api/memstats", s.handleMemStats)
	s.mux.HandleFunc("/api/timers", s.handleTimers)
	s.mux.HandleFunc("/api/queries", s.handleQueries)
	s.mux.HandleFunc("/api/timeline", s.handleTimeline)
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/environ", s.handleEnviron)
	s.mux.HandleFunc("/api/deps", s.handleDeps)
	s.mux.HandleFunc("/api/errors", s.handleErrors)
	s.mux.HandleFunc("/api/profiles", s.handleProfiles)
	s.mux.HandleFunc("/api/profiles/capture", s.handleProfileCapture)
	s.mux.HandleFunc("/api/profiles/download", s.handleProfileDownload)
	s.mux.HandleFunc("/api/outgoing", s.handleOutgoing)
	s.mux.HandleFunc("/api/caches", s.handleCaches)
	s.mux.HandleFunc("/api/ratelimits", s.handleRateLimits)
	s.mux.HandleFunc("/api/benchmarks", s.handleBenchmarks)
	s.mux.HandleFunc("/api/overview", s.handleOverview)
	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.HandleFunc("/events", s.handleSSE)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Logger == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.Logger.History())
}

func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Middleware == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.Middleware.LastRequests(100))
}

func (s *Server) handleGoroutines(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Goroutine == nil {
		jsonResponse(w, map[string]any{"count": 0})
		return
	}
	jsonResponse(w, s.providers.Goroutine())
}

func (s *Server) handleMemStats(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.MemStats == nil {
		jsonResponse(w, map[string]any{})
		return
	}
	jsonResponse(w, s.providers.MemStats())
}

func (s *Server) handleTimers(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Timer == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.Timer.All())
}

func (s *Server) handleQueries(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.DBLogger == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.DBLogger.LastQueries(100))
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Timeline == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.Timeline.LastEvents(200))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Config == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.Config.Snapshot())
}

// Phase 5 handlers

func (s *Server) handleEnviron(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Environ == nil {
		jsonResponse(w, map[string]any{})
		return
	}
	jsonResponse(w, s.providers.Environ())
}

func (s *Server) handleDeps(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Deps == nil {
		jsonResponse(w, map[string]any{})
		return
	}
	jsonResponse(w, s.providers.Deps())
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.ErrTracker == nil {
		jsonResponse(w, map[string]any{})
		return
	}
	jsonResponse(w, s.providers.ErrTracker.Stats())
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Profiler == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.Profiler.Profiles())
}

func (s *Server) handleProfileCapture(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Profiler == nil {
		http.Error(w, "profiler not available", http.StatusServiceUnavailable)
		return
	}

	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "heap"
	}

	var prof *profiler.Profile
	var err error

	switch typ {
	case "cpu":
		durationStr := r.URL.Query().Get("duration")
		duration := 10 * time.Second
		if durationStr != "" {
			if d, parseErr := time.ParseDuration(durationStr); parseErr == nil {
				duration = d
			}
		}
		if duration > 60*time.Second {
			duration = 60 * time.Second
		}
		// Run CPU capture in background, return immediately
		go func() {
			s.providers.Profiler.CaptureCPU(duration)
		}()
		jsonResponse(w, map[string]any{"status": "capturing", "type": "cpu", "duration": duration.String()})
		return
	case "heap":
		prof, err = s.providers.Profiler.CaptureHeap()
	case "goroutine":
		prof, err = s.providers.Profiler.CaptureGoroutine()
	case "mutex":
		prof, err = s.providers.Profiler.CaptureMutex()
	case "block":
		prof, err = s.providers.Profiler.CaptureBlock()
	case "allocs":
		prof, err = s.providers.Profiler.CaptureAllocs()
	default:
		http.Error(w, "unknown profile type: "+typ, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, prof)
}

func (s *Server) handleProfileDownload(w http.ResponseWriter, r *http.Request) {
	if s.providers.Profiler == nil {
		http.Error(w, "profiler not available", http.StatusServiceUnavailable)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	data, ok := s.providers.Profiler.ProfileData(id)
	if !ok {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pprof", id))
	w.Write(data)
}

// Phase 6 handlers

func (s *Server) handleOutgoing(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.HTTPTracer == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.HTTPTracer.LastTraces(100))
}

func (s *Server) handleCaches(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.CacheMon == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.CacheMon.Stats())
}

func (s *Server) handleRateLimits(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.RateMon == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.RateMon.Stats())
}

func (s *Server) handleBenchmarks(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if s.providers.Bench == nil {
		jsonResponse(w, []any{})
		return
	}
	jsonResponse(w, s.providers.Bench.Results())
}

type overviewData struct {
	LogCount       int    `json:"log_count"`
	RequestCount   int    `json:"request_count"`
	GoroutineCount int    `json:"goroutine_count"`
	HeapAlloc      string `json:"heap_alloc"`
	TimerCount     int    `json:"timer_count"`
	QueryCount     int    `json:"query_count"`
	TimelineCount  int    `json:"timeline_count"`
	ConfigCount    int    `json:"config_count"`
	ErrorCount     int    `json:"error_count"`
	GoVersion      string `json:"go_version"`
	Uptime         string `json:"uptime"`
	ProfileCount   int    `json:"profile_count"`
	DepCount       int    `json:"dep_count"`
	WSClients      int    `json:"ws_clients"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	data := overviewData{}

	if s.providers.Logger != nil {
		data.LogCount = len(s.providers.Logger.History())
	}
	if s.providers.Middleware != nil {
		data.RequestCount = s.providers.Middleware.Count()
	}
	if s.providers.Goroutine != nil {
		snap := s.providers.Goroutine()
		data.GoroutineCount = snap.Count
	}
	if s.providers.MemStats != nil {
		snap := s.providers.MemStats()
		data.HeapAlloc = snap.HeapAllocStr()
	}
	if s.providers.Timer != nil {
		data.TimerCount = len(s.providers.Timer.All())
	}
	if s.providers.DBLogger != nil {
		data.QueryCount = s.providers.DBLogger.Count()
	}
	if s.providers.Timeline != nil {
		data.TimelineCount = s.providers.Timeline.Count()
	}
	if s.providers.Config != nil {
		data.ConfigCount = len(s.providers.Config.Names())
	}
	if s.providers.ErrTracker != nil {
		data.ErrorCount = s.providers.ErrTracker.Count()
	}
	if s.providers.Environ != nil {
		env := s.providers.Environ()
		data.GoVersion = env.GoVersion
		data.Uptime = env.UptimeStr()
	}
	if s.providers.Profiler != nil {
		data.ProfileCount = s.providers.Profiler.Count()
	}
	if s.providers.Deps != nil {
		d := s.providers.Deps()
		data.DepCount = d.Total
	}
	data.WSClients = s.hub.clientCount()

	jsonResponse(w, data)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		http.Error(w, "Expected WebSocket upgrade", http.StatusBadRequest)
		return
	}

	ws, err := upgradeWebSocket(w, r)
	if err != nil {
		stdlog.Printf("[godevtool] WebSocket upgrade failed: %v", err)
		return
	}
	stdlog.Printf("[godevtool] WebSocket client connected from %s", r.RemoteAddr)

	c := &client{send: make(chan []byte, 256)}
	s.hub.register(c)

	// writer goroutine — sends messages from hub to client
	go func() {
		defer ws.Close()
		for msg := range c.send {
			if err := ws.writeTextFrame(msg); err != nil {
				return
			}
		}
	}()

	// reader goroutine — reads from client (handles ping/pong/close)
	// When the reader exits, unregister closes c.send which stops the writer.
	go func() {
		defer func() {
			stdlog.Printf("[godevtool] WebSocket reader exiting for %s", r.RemoteAddr)
			s.hub.unregister(c)
			ws.Close()
		}()
		for {
			opcode, data, err := ws.readFrame()
			if err != nil {
				stdlog.Printf("[godevtool] WebSocket read error from %s: %v", r.RemoteAddr, err)
				return
			}
			switch opcode {
			case 0x8: // close
				ws.writeCloseFrame()
				return
			case 0x9: // ping
				ws.writePongFrame(data)
			}
		}
	}()
}

// handleSSE implements Server-Sent Events — a simple, reliable alternative to
// WebSocket for server→client push. Works over plain HTTP, no Hijack needed.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Register this client with the hub
	c := &client{send: make(chan []byte, 256)}
	s.hub.register(c)
	defer s.hub.unregister(c)

	stdlog.Printf("[godevtool] SSE client connected from %s", r.RemoteAddr)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			// SSE format: "data: <json>\n\n"
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			stdlog.Printf("[godevtool] SSE client disconnected from %s", r.RemoteAddr)
			return
		}
	}
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
