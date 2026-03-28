package dashboard

import (
	"encoding/json"
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

	conn, bufrw, err := upgradeWebSocket(w, r)
	if err != nil {
		return
	}

	c := &client{send: make(chan []byte, 256)}
	s.hub.register(c)

	// writer goroutine
	go func() {
		defer conn.Close()
		for msg := range c.send {
			if err := writeTextFrame(bufrw, msg); err != nil {
				return
			}
		}
	}()

	// reader goroutine (handles ping/close)
	go func() {
		defer s.hub.unregister(c)
		for {
			opcode, data, err := readFrame(bufrw)
			if err != nil {
				return
			}
			switch opcode {
			case 0x8: // close
				writeCloseFrame(bufrw)
				return
			case 0x9: // ping
				writePongFrame(bufrw, data)
			}
		}
	}()
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
