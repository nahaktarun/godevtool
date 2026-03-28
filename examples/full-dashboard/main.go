package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/nahaktarun/godevtool"
	"github.com/nahaktarun/godevtool/log"
	"github.com/nahaktarun/godevtool/timeline"
)

// AppConfig demonstrates the config viewer with redaction.
type AppConfig struct {
	Host        string   `json:"host" env:"APP_HOST"`
	Port        int      `json:"port" env:"APP_PORT"`
	Debug       bool     `json:"debug"`
	DatabaseURL string   `devtool:"redact" env:"DATABASE_URL"`
	APIKey      string   `devtool:"redact" env:"API_KEY"`
	Features    []string `json:"features"`
	MaxWorkers  int      `json:"max_workers"`
}

func main() {
	dt := godevtool.New(
		godevtool.WithAppName("full-demo"),
		godevtool.WithLogLevel(log.LevelDebug),
	)
	defer dt.Shutdown()

	// Register application configuration
	appCfg := AppConfig{
		Host:        "localhost",
		Port:        8080,
		Debug:       true,
		DatabaseURL: "postgres://user:secret@db.internal:5432/myapp",
		APIKey:      "sk-prod-xxxxxxxxxxxx",
		Features:    []string{"auth", "cache", "rate-limit"},
		MaxWorkers:  10,
	}
	dt.RegisterConfig("Application", appCfg)
	dt.RegisterConfig("Database", struct {
		Driver      string `json:"driver"`
		MaxOpenConn int    `json:"max_open_conn"`
		MaxIdleConn int    `json:"max_idle_conn"`
		Password    string `devtool:"redact"`
	}{
		Driver:      "postgres",
		MaxOpenConn: 25,
		MaxIdleConn: 5,
		Password:    "super-secret",
	})

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
		// Timeline span for the whole request
		span := dt.TimelineStart(timeline.CatHTTP, "GET /api/users", nil)
		defer span.End()

		// Simulate a database query
		dbSpan := dt.TimelineStart(timeline.CatDB, "SELECT * FROM users", nil)
		time.Sleep(time.Duration(10+rand.Intn(30)) * time.Millisecond)
		dt.DBLogger().Record("SELECT * FROM users ORDER BY created_at DESC LIMIT 10", nil,
			time.Duration(10+rand.Intn(30))*time.Millisecond, nil, 3)
		dbSpan.SetData("rows", 3)
		dbSpan.End()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"users": []map[string]string{
				{"name": "Alice", "email": "alice@example.com"},
				{"name": "Bob", "email": "bob@example.com"},
				{"name": "Charlie", "email": "charlie@example.com"},
			},
		})
		span.SetData("status", 200)
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		defer dt.Timer("GET /api/health").Stop()
		dt.TimelineRecord(timeline.CatHTTP, "GET /api/health", map[string]any{"status": 200})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	mux.HandleFunc("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		defer dt.Timer("GET /api/slow").Stop()
		span := dt.TimelineStart(timeline.CatHTTP, "GET /api/slow", nil)
		defer span.End()

		// Simulate a slow database query
		dt.DBLogger().Record("SELECT * FROM large_table WHERE complex_condition = true", nil,
			450*time.Millisecond, nil, 1500)
		time.Sleep(500 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "done"})
		span.SetData("status", 200)
	})

	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		dt.Log.Error("simulated error", fmt.Errorf("something went wrong"), "endpoint", "/api/error")
		dt.TimelineRecord(timeline.CatHTTP, "GET /api/error", map[string]any{"status": 500, "error": "something went wrong"})

		// Simulate a failed query
		dt.DBLogger().Record("INSERT INTO audit_log (event) VALUES ($1)", []any{"error"},
			5*time.Millisecond, fmt.Errorf("connection refused"), 0)

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	})

	// Wrap with devtool middleware for request capture
	handler := dt.Middleware().Handler(mux)

	// Generate some background activity with timeline events
	go func() {
		time.Sleep(2 * time.Second)
		for {
			dt.Log.Info("background task running", "iteration", rand.Intn(1000))
			dt.TimelineRecord(timeline.CatCustom, "background-task", map[string]any{
				"iteration": rand.Intn(1000),
			})
			time.Sleep(5 * time.Second)
		}
	}()

	// Print config to terminal
	fmt.Println()
	dt.PrintConfig()

	fmt.Println()
	fmt.Println("  Application: http://localhost:8080")
	fmt.Println("  Dashboard:   http://localhost:9999")
	fmt.Println()
	fmt.Println("  Endpoints:")
	fmt.Println("    curl http://localhost:8080/api/users     (with DB queries)")
	fmt.Println("    curl http://localhost:8080/api/health    (fast)")
	fmt.Println("    curl http://localhost:8080/api/slow      (slow DB query)")
	fmt.Println("    curl http://localhost:8080/api/error     (error + failed query)")
	fmt.Println()
	fmt.Println("  Open http://localhost:9999 for the full dashboard!")
	fmt.Println()

	if err := http.ListenAndServe(":8080", handler); err != nil {
		dt.Log.Error("server failed", err)
	}
}
