package dashboard

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHandleOverviewIncludesVersion(t *testing.T) {
	s := NewServer(":0", DataProviders{Version: "v1.2.3"})
	req := httptest.NewRequest("GET", "/api/overview", nil)
	w := httptest.NewRecorder()

	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var data map[string]any
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if got, ok := data["version"].(string); !ok || got != "v1.2.3" {
		t.Fatalf("version = %v, want %q", data["version"], "v1.2.3")
	}
}
