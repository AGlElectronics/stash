package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestAuditLogRedactsGuestDataWithoutLostfoundView(t *testing.T) {
	pool := testdb.New(t)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO audit_log (action, entity_type, entity_id, after_json, note)
		VALUES ('lostfound.create', 'lostfound', $1, '{"item_name":"Wallet","contact_name":"Alex Example","contact_info":"alex@example.org"}', '')`,
		uuid.New()); err != nil {
		t.Fatal(err)
	}
	list := func(perms ...string) string {
		set := map[string]struct{}{}
		for _, p := range perms {
			set[p] = struct{}{}
		}
		req := httptest.NewRequest(http.MethodGet, "/?limit=999999&offset=-3", nil)
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: uuid.New(), Permissions: set}))
		rec := httptest.NewRecorder()
		listAuditLog(pool)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	readonly := list(domain.PermAuditView)
	if strings.Contains(readonly, "Alex Example") || strings.Contains(readonly, "alex@example.org") {
		t.Fatalf("readonly sees guest data: %s", readonly)
	}
	if !strings.Contains(readonly, "Wallet") || !strings.Contains(readonly, `"limit":500`) || !strings.Contains(readonly, `"offset":0`) {
		t.Fatalf("unexpected body: %s", readonly)
	}
	if lead := list(domain.PermAuditView, domain.PermLostfoundView); !strings.Contains(lead, "Alex Example") {
		t.Fatalf("lostfound.view should see the contact: %s", lead)
	}
}

func TestCSVSafe(t *testing.T) {
	cases := map[string]string{
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+cmd":              "'+cmd",
		"-1+2":              "'-1+2",
		"@SUM(A1)":          "'@SUM(A1)",
		"\tcmd":             "'\tcmd",
		"-5":                "-5",
		"+3.25":             "+3.25",
		"plain":             "plain",
		"":                  "",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSafeCSVWriter(t *testing.T) {
	var buf bytes.Buffer
	w := newSafeCSV(&buf)
	_ = w.Write([]string{"=1+1", "ok"})
	w.Flush()
	if got := buf.String(); got != "'=1+1,ok\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClampPage(t *testing.T) {
	cases := []struct{ lim, off, max, wantLim, wantOff int32 }{
		{100, 0, 500, 100, 0},
		{0, -5, 500, 1, 0},
		{-1, 10, 500, 1, 10},
		{100000, 3, 500, 500, 3},
	}
	for _, c := range cases {
		l, o := clampPage(c.lim, c.off, c.max)
		if l != c.wantLim || o != c.wantOff {
			t.Errorf("clampPage(%d,%d,%d) = %d,%d", c.lim, c.off, c.max, l, o)
		}
	}
}
