package dashboard

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

// Server serves the dashboard UI and API endpoints.
type Server struct {
	addr       string
	httpServer *http.Server
	hub        *Hub
	providers  DataProviders
	mux        *http.ServeMux
}

// NewServer creates a dashboard server.
func NewServer(addr string, providers DataProviders) *Server {
	s := &Server{
		addr:      addr,
		hub:       newHub(),
		providers: providers,
		mux:       http.NewServeMux(),
	}

	s.registerAPIRoutes()

	// serve embedded static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("godevtool: failed to load static files: %v", err))
	}
	s.mux.Handle("/", http.FileServer(http.FS(staticFS)))

	return s
}

// Start begins serving the dashboard. Non-blocking.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("godevtool dashboard: %w", err)
	}

	go s.httpServer.Serve(ln)
	return nil
}

// Stop gracefully shuts down the dashboard server.
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// Hub returns the WebSocket hub for broadcasting events.
func (s *Server) Hub() *Hub {
	return s.hub
}

// Addr returns the configured address.
func (s *Server) Addr() string {
	return s.addr
}
