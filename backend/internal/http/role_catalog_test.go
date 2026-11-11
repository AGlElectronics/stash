package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

const testHRToken = "test-hr-service-token-0123456789abcdef"

// denyAll stands in for the sign-in middleware: it refuses everyone.
func denyAll(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httperror.Render(w, r, httperror.ErrUnauthorized)
	})
}

func TestRoleCatalogNeedsHRTokenOrSignIn(t *testing.T) {
	h := roleCatalogAuth(testHRToken, denyAll, roleCatalogHandler(auth.DefaultAccessConfig()))
	cases := []struct {
		name, header string
		want         int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"wrong token", "Bearer not-the-token", http.StatusUnauthorized},
		{"token without Bearer", testHRToken, http.StatusUnauthorized},
		{"HR service token", "Bearer " + testHRToken, http.StatusOK},
		{"scheme in lower case", "bearer " + testHRToken, http.StatusOK},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: %d %s, want %d", tc.name, rec.Code, rec.Body.String(), tc.want)
		}
	}

	// Without HR_SERVICE_TOKEN even the right-looking token needs a sign-in.
	none := roleCatalogAuth("", denyAll, roleCatalogHandler(auth.DefaultAccessConfig()))
	req := httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil)
	req.Header.Set("Authorization", "Bearer "+testHRToken)
	rec := httptest.NewRecorder()
	none(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no HR_SERVICE_TOKEN configured: %d, want 401", rec.Code)
	}
}

func TestRoleCatalogETag(t *testing.T) {
	h := roleCatalogHandler(auth.DefaultAccessConfig())
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil))
	etag := rec.Header().Get("ETag")
	if rec.Code != http.StatusOK || etag == "" {
		t.Fatalf("first read: %d, ETag %q", rec.Code, etag)
	}
	for _, inm := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
		req := httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil)
		req.Header.Set("If-None-Match", inm)
		rec = httptest.NewRecorder()
		h(rec, req)
		if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
			t.Errorf("If-None-Match %s: %d, want 304 without a body", inm, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil)
	req.Header.Set("If-None-Match", `"stale"`)
	rec = httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("stale ETag: %d, want 200", rec.Code)
	}
}

func TestRoleCatalogGroupFromBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.toml")
	if err := os.WriteFile(path, []byte("[[bindings]]\ngroup = \"/Teams/Stash-Admins\"\nrole = \"admin\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	access, err := auth.LoadAccessConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	roleCatalogHandler(access)(rec, httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil))
	var body struct {
		Revision string             `json:"revision"`
		Roles    []roleCatalogEntry `json:"roles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, r := range body.Roles {
		if r.Key == "stash_admin" {
			if r.Group != "teams/stash-admins" || r.GroupDisplay != "/Teams/Stash-Admins" || !r.Privileged {
				t.Fatalf("admin entry %+v", r)
			}
			return
		}
	}
	t.Fatal("stash_admin missing")
}
