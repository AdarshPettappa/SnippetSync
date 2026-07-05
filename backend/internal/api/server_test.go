package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchModules(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=kafka", nil)
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "Kafka Producer") || !strings.Contains(res.Body.String(), "Kafka Consumer") {
		t.Fatalf("expected kafka modules in response: %s", res.Body.String())
	}
}

func TestGenerateProjectExpandsDependencies(t *testing.T) {
	server := NewServer()
	body := strings.NewReader(`{"project_name":"demo","module_ids":["rate-limiter"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/generate", body)
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Redis Cache") {
		t.Fatalf("expected dependency summary to include Redis Cache: %s", res.Body.String())
	}
}
