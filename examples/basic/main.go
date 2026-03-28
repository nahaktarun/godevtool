package main

import (
	"fmt"
	"time"

	"github.com/tarunnahak/godevtool"
	"github.com/tarunnahak/godevtool/log"
)

type Address struct {
	Street string
	City   string
	State  string
	Zip    string
}

type User struct {
	ID        int
	Name      string
	Email     string
	Active    bool
	Roles     []string
	Address   Address
	Tags      map[string]string
	CreatedAt time.Time
}

func main() {
	// Create a devtool instance with debug-level logging
	dt := godevtool.New(
		godevtool.WithAppName("demo"),
		godevtool.WithLogLevel(log.LevelDebug),
	)

	// === Structured Logging ===
	fmt.Println("=== Structured Logging ===")
	fmt.Println()

	dt.Log.Info("application starting", "version", "1.0.0", "port", 8080)
	dt.Log.Debug("configuration loaded", "env", "development", "debug", true)
	dt.Log.Warn("cache miss rate high", "rate", "45%", "threshold", "30%")
	dt.Log.Error("connection failed", fmt.Errorf("dial tcp: connection refused"), "host", "db.local", "port", 5432)

	// Child logger with context
	reqLog := dt.Log.With("request_id", "abc-123", "user_id", 42)
	reqLog.Info("processing request", "method", "GET", "path", "/api/users")
	reqLog.Debug("query executed", "rows", 15, "duration", "2.3ms")

	fmt.Println()

	// === Variable Inspector ===
	fmt.Println("=== Variable Inspector ===")
	fmt.Println()

	user := User{
		ID:     1,
		Name:   "Alice Johnson",
		Email:  "alice@example.com",
		Active: true,
		Roles:  []string{"admin", "editor", "viewer"},
		Address: Address{
			Street: "123 Main St",
			City:   "Springfield",
			State:  "IL",
			Zip:    "62701",
		},
		Tags: map[string]string{
			"department": "engineering",
			"team":       "platform",
		},
		CreatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	dt.Inspect(user)
	fmt.Println()

	// Inspect a slice
	dt.Inspect([]int{10, 20, 30, 40, 50})
	fmt.Println()

	// Inspect a nil pointer
	var nilUser *User
	dt.Inspect(nilUser)
	fmt.Println()

	// === Execution Timer ===
	fmt.Println()
	fmt.Println("=== Execution Timer ===")
	fmt.Println()

	// Time a simulated operation
	func() {
		defer dt.Timer("database-query").Stop()
		time.Sleep(50 * time.Millisecond)
	}()

	func() {
		defer dt.Timer("api-call").Stop()
		time.Sleep(100 * time.Millisecond)
	}()

	// Run database-query again to show aggregation
	func() {
		defer dt.Timer("database-query").Stop()
		time.Sleep(30 * time.Millisecond)
	}()

	fmt.Println()
	fmt.Println("Timer Report:")
	dt.PrintTimerReport()

	// === Stack Trace ===
	fmt.Println()
	fmt.Println("=== Stack Trace ===")
	fmt.Println()

	dt.PrintStack()

	// === Disable for production ===
	fmt.Println()
	fmt.Println("=== Disable/Enable ===")
	dt.Disable()
	dt.Log.Info("this should NOT appear")
	dt.Enable()
	dt.Log.Info("this SHOULD appear", "status", "re-enabled")
}
