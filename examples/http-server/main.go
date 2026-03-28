package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tarunnahak/godevtool"
	"github.com/tarunnahak/godevtool/log"
)

func main() {
	dt := godevtool.New(
		godevtool.WithAppName("http-demo"),
		godevtool.WithLogLevel(log.LevelDebug),
	)
	defer dt.Shutdown()

	// Start background monitors
	dt.StartGoroutineMonitor(2 * time.Second)
	dt.StartMemStats(2 * time.Second)

	// Set up routes
	mux := http.NewServeMux()

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		defer dt.Timer("GET /api/users").Stop()
		time.Sleep(20 * time.Millisecond) // simulate work

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"users": []map[string]string{
				{"name": "Alice", "email": "alice@example.com"},
				{"name": "Bob", "email": "bob@example.com"},
			},
		})
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// Debug endpoints powered by godevtool
	mux.HandleFunc("/debug/requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dt.Middleware().LastRequests(20))
	})

	mux.HandleFunc("/debug/goroutines", func(w http.ResponseWriter, r *http.Request) {
		snap := dt.GoroutineSnapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snap)
	})

	mux.HandleFunc("/debug/memstats", func(w http.ResponseWriter, r *http.Request) {
		snap := dt.MemSnapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snap)
	})

	mux.HandleFunc("/debug/timers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dt.TimerReport().All())
	})

	// Wrap with devtool middleware
	handler := dt.Middleware().Handler(mux)

	dt.Log.Info("server starting", "addr", ":8080")
	fmt.Println()
	fmt.Println("Endpoints:")
	fmt.Println("  http://localhost:8080/api/users")
	fmt.Println("  http://localhost:8080/api/health")
	fmt.Println()
	fmt.Println("Debug endpoints:")
	fmt.Println("  http://localhost:8080/debug/requests")
	fmt.Println("  http://localhost:8080/debug/goroutines")
	fmt.Println("  http://localhost:8080/debug/memstats")
	fmt.Println("  http://localhost:8080/debug/timers")
	fmt.Println()

	// Print initial stats
	dt.PrintMemStats()
	fmt.Println()
	dt.PrintGoroutines()

	if err := http.ListenAndServe(":8080", handler); err != nil {
		dt.Log.Error("server failed", err)
	}
}
