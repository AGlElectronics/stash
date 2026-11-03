package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealth_NotConfigured(t *testing.T) {
	t.Parallel()

	h := newHealthHandler(nil, "test-version")
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	h.get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when db is not configured, got %d", w.Code)
	}
	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %q", resp.Status)
	}
	if resp.Database != "not_configured" {
		t.Errorf("expected database=not_configured, got %q", resp.Database)
	}
	if resp.Version != "test-version" {
		t.Errorf("expected version=test-version, got %q", resp.Version)
	}
}

func TestHealth_UptimeIncreases(t *testing.T) {
	t.Parallel()

	h := newHealthHandler(nil, "v")
	h.startedAt = time.Now().Add(-2 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	h.get(w, req)

	var resp healthResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.UptimeSec < 1 {
		t.Errorf("expected uptime >=1 sec, got %d", resp.UptimeSec)
	}
}
