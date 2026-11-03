package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type healthHandler struct {
	pool      *pgxpool.Pool
	startedAt time.Time
	version   string
}

type healthResponse struct {
	Status    string `json:"status"`
	Database  string `json:"database"`
	Version   string `json:"version"`
	UptimeSec int64  `json:"uptime_sec"`
}

func newHealthHandler(pool *pgxpool.Pool, version string) *healthHandler {
	return &healthHandler{
		pool:      pool,
		startedAt: time.Now(),
		version:   version,
	}
}

func (h *healthHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "ok"
	if h.pool != nil {
		if err := h.pool.Ping(ctx); err != nil {
			dbStatus = "down"
		}
	} else {
		dbStatus = "not_configured"
	}

	overall := "ok"
	if dbStatus == "down" {
		overall = "degraded"
	}

	resp := healthResponse{
		Status:    overall,
		Database:  dbStatus,
		Version:   h.version,
		UptimeSec: int64(time.Since(h.startedAt).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if overall != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(resp)
}
