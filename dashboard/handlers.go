package dashboard

import (
	"encoding/json"
	"fmt"
	stdlog "log"
	"net/http"

	"github.com/tarunnahak/godevtool/config"
	"github.com/tarunnahak/godevtool/dblog"
	"github.com/tarunnahak/godevtool/goroutine"
	"github.com/tarunnahak/godevtool/log"
	"github.com/tarunnahak/godevtool/memstats"
	"github.com/tarunnahak/godevtool/middleware"
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

type overviewData struct {
	LogCount       int    `json:"log_count"`
	RequestCount   int    `json:"request_count"`
	GoroutineCount int    `json:"goroutine_count"`
	HeapAlloc      string `json:"heap_alloc"`
	TimerCount     int    `json:"timer_count"`
	QueryCount     int    `json:"query_count"`
	TimelineCount  int    `json:"timeline_count"`
	ConfigCount    int    `json:"config_count"`
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
