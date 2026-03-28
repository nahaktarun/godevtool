package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/tarunnahak/godevtool"
	"github.com/tarunnahak/godevtool/log"
)

func main() {
	dt := godevtool.New(
		godevtool.WithAppName("full-demo"),
		godevtool.WithLogLevel(log.LevelDebug),
	)
	defer dt.Shutdown()

	// Start background monitors
	dt.StartGoroutineMonitor(3 * time.Second)
	dt.StartMemStats(3 * time.Second)

	// Start the dashboard on port 9999
	if err := dt.StartDashboard(":9999"); err != nil {
		dt.Log.Error("failed to start dashboard", err)
		return
	}

	// Application routes
	mux := http.NewServeMux()

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		defer dt.Timer("GET /api/users").Stop()
		// simulate variable latency
		time.Sleep(time.Duration(20+rand.Intn(80)) * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"users": []map[string]string{
				{"name": "Alice", "email": "alice@example.com"},
				{"name": "Bob", "email": "bob@example.com"},
				{"name": "Charlie", "email": "charlie@example.com"},
			},
		})
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		defer dt.Timer("GET /api/health").Stop()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	mux.HandleFunc("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		defer dt.Timer("GET /api/slow").Stop()
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "done"})
	})

	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		dt.Log.Error("simulated error", fmt.Errorf("something went wrong"), "endpoint", "/api/error")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	})

	// Wrap with devtool middleware for request capture
	handler := dt.Middleware().Handler(mux)

	// Generate some background activity
	go func() {
		time.Sleep(2 * time.Second)
		for {
			dt.Log.Info("background task running", "iteration", rand.Intn(1000))
			time.Sleep(5 * time.Second)
		}
	}()

	fmt.Println()
	fmt.Println("  Application: http://localhost:8080")
	fmt.Println("  Dashboard:   http://localhost:9999")
	fmt.Println()
	fmt.Println("  Try these endpoints:")
	fmt.Println("    curl http://localhost:8080/api/users")
	fmt.Println("    curl http://localhost:8080/api/health")
	fmt.Println("    curl http://localhost:8080/api/slow")
	fmt.Println("    curl http://localhost:8080/api/error")
	fmt.Println()
	fmt.Println("  Then open the dashboard to see real-time data!")
	fmt.Println()

	if err := http.ListenAndServe(":8080", handler); err != nil {
		dt.Log.Error("server failed", err)
	}
}
