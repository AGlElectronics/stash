package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestPublicLostFoundShowsOnlyMinimalDetails(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO lost_found_reports (report_number, type, status, item_name, description, location, contact_name, contact_info)
		VALUES ('LF-0001', 'found', 'open',     'Blue backpack', 'has a name tag: Alex', 'Hall B', 'Finder', 'finder@example.org'),
		       ('LF-0002', 'found', 'returned', 'Keys',          '',                     '',       '',       ''),
		       ('LF-0003', 'lost',  'open',     'Phone',         'black',                '',       'Owner',  '+43 660 1')`); err != nil {
		t.Fatal(err)
	}
	h := &publicLostFoundHandler{pool: pool}
	r := chi.NewRouter()
	r.Get("/api/public/lostfound/", h.search)
	r.Get("/api/public/lostfound/{reportNumber}", h.getByNumber)

	get := func(path string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	code, body := get("/api/public/lostfound/LF-0001")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(body) != 3 || body["item_name"] != "Blue backpack" || body["report_number"] != "LF-0001" {
		t.Fatalf("public report exposes more than number, name and day: %v", body)
	}
	if d, _ := body["date_reported"].(string); len(d) != len("2006-01-02") {
		t.Fatalf("date_reported should be a day, got %q", d)
	}

	for _, hidden := range []string{"LF-0002", "LF-0003"} {
		if code, _ := get("/api/public/lostfound/" + hidden); code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404 (only open found reports are public)", hidden, code)
		}
	}

	_, list := get("/api/public/lostfound/?q=alex")
	if list["total"].(float64) != 0 {
		t.Errorf("search must not match the description: %v", list)
	}
	_, list = get("/api/public/lostfound/?q=backpack&limit=100000")
	if list["total"].(float64) != 1 {
		t.Errorf("search by item name: %v", list)
	}
}
