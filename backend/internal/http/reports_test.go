package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestPlannedReports(t *testing.T) {
	pool := testdb.New(t)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO items (name, item_type, quantity_owned, quantity_damaged, minimum_quantity)
		VALUES ('=cmd|calc', 'consumable', 2, 0, 5), ('Plenty', 'consumable', 10, 1, 5)`); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/api/export/reports/{report}.csv", exportPlannedReport(pool))
	get := func(path string, perms ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(testdb.Actor(t, pool, "Exporter", perms...))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := get("/api/export/reports/low-stock.csv", domain.PermItemView); rec.Code != http.StatusForbidden {
		t.Fatalf("without export.create: %d", rec.Code)
	}
	rec := get("/api/export/reports/low-stock.csv", domain.PermExportCreate)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "'=cmd|calc") || strings.Contains(body, "Plenty") {
		t.Fatalf("low-stock export: %q", body)
	}
	if rec := get("/api/export/reports/damaged.csv", domain.PermExportCreate); !strings.Contains(rec.Body.String(), "Plenty") {
		t.Fatalf("damaged export: %q", rec.Body.String())
	}
	if rec := get("/api/export/reports/nope.csv", domain.PermExportCreate); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown report: %d", rec.Code)
	}
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE action = 'export.report'`).Scan(&n)
	if n != 2 {
		t.Fatalf("export audit rows = %d, want 2", n)
	}
}
