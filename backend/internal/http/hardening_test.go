package http

// Shared foundations in Stash: authbase security headers, CORS, body limit
// and rate limits, and denlink's revoked state.

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/config"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func testRouter(cfg *config.Config) http.Handler {
	den := denlink.New(nil, cfg.DenConfig("test"), zerolog.Nop())
	return NewRouter(Deps{Config: cfg, Logger: zerolog.Nop(), Den: den, Access: auth.DefaultAccessConfig()}, "test")
}

func devConfig() *config.Config {
	return &config.Config{AuthMode: config.AuthModeDev, AppEnv: "development", CORSAllowedOrigins: []string{"https://stash.example.org"}}
}

func oidcConfig() *config.Config {
	return &config.Config{AuthMode: config.AuthModeOIDC, AppEnv: "production", CORSAllowedOrigins: []string{"https://stash.example.org"}}
}

func TestSecurityHeadersAndCORS(t *testing.T) {
	h := testRouter(oidcConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/den/state", nil)
	req.Header.Set("Origin", "https://stash.example.org")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("state: %d", rec.Code)
	}
	for k, v := range map[string]string{
		"X-Content-Type-Options":        "nosniff",
		"Referrer-Policy":               "no-referrer",
		"X-Frame-Options":               "DENY",
		"Content-Security-Policy":       "default-src 'none'; frame-ancestors 'none'",
		"Access-Control-Allow-Origin":   "https://stash.example.org",
		"Access-Control-Expose-Headers": "X-Request-ID",
	} {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}

	preflight := func(h http.Handler) string {
		req := httptest.NewRequest(http.MethodOptions, "/api/items", nil)
		req.Header.Set("Origin", "https://stash.example.org")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Header().Get("Access-Control-Allow-Headers")
	}
	if got := preflight(testRouter(devConfig())); !strings.Contains(got, "X-Dev-User") {
		t.Errorf("dev mode in development should allow X-Dev-User, got %q", got)
	}
	if got := preflight(h); strings.Contains(got, "X-Dev") || !strings.Contains(got, "Authorization") {
		t.Errorf("OIDC mode must not allow dev headers, got %q", got)
	}
}

func TestDecodeJSONLimits(t *testing.T) {
	decode := func(body string, limit int64) *httperror.Error {
		var dst struct {
			Name string `json:"name"`
		}
		err := decodeJSONLimit(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), &dst, limit)
		var he *httperror.Error
		if err != nil && !errors.As(err, &he) {
			t.Fatalf("%q: untyped error %v", body, err)
		}
		return he
	}
	if e := decode(`{"name":"`+strings.Repeat("x", 2<<20)+`"}`, 1<<20); e == nil || e.Code != httperror.CodeRequestTooLarge {
		t.Errorf("oversized body: %v", e)
	}
	for _, body := range []string{``, `{"bogus":1}`, `{"name":1}`, `{"name":"a"} {"name":"b"}`, `{`} {
		if e := decode(body, 1<<20); e == nil || e.Code != httperror.CodeBadRequest {
			t.Errorf("%q: %v, want bad_request", body, e)
		}
	}
	if e := decode(`{"name":"ok"}`, 1<<20); e != nil {
		t.Errorf("valid body: %v", e)
	}

	rec := httptest.NewRecorder()
	httperror.Render(rec, httptest.NewRequest(http.MethodPost, "/", nil), decode(`{"name":"`+strings.Repeat("x", 100)+`"}`, 10))
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), `"request_too_large"`) {
		t.Errorf("rendered: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRoleCatalogRouteAndRateLimit(t *testing.T) {
	cfg := oidcConfig()
	cfg.HRServiceToken = testHRToken
	h := testRouter(cfg)
	get := func(authz string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil)
		req.RemoteAddr = "203.0.113.9:4000"
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := get("Bearer " + testHRToken); rec.Code != http.StatusOK {
		t.Fatalf("HR's service token: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(""); rec.Code == http.StatusOK {
		t.Fatal("an anonymous request read the catalog")
	}
	var last *httptest.ResponseRecorder
	for range 30 {
		last = get("Bearer " + testHRToken)
	}
	if last.Code != http.StatusTooManyRequests || last.Header().Get("Retry-After") == "" || !strings.Contains(last.Body.String(), `"rate_limited"`) {
		t.Fatalf("after a burst: %d %s", last.Code, last.Body.String())
	}
}

func TestDenRevokedState(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO den_link (den_base_url, credential, bundle, last_error, revoked_at)
		VALUES ('https://den.test', 'cred', '{"brand":{},"settings":{"data":{}}}', 'Den rejected this module''s credential', now())`); err != nil {
		t.Fatalf("seed a revoked link: %v", err)
	}
	cfg := devConfig()
	cfg.Den = denlink.Config{AppEnv: "development"}
	den := denlink.New(pool, cfg.DenConfig("test"), zerolog.Nop())
	if err := den.Restore(ctx); err != nil {
		t.Fatalf("restore: %v", err)
	}
	h := NewRouter(Deps{Config: cfg, Pool: pool, Logger: zerolog.Nop(), Den: den, Access: auth.DefaultAccessConfig()}, "test")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/den/state", nil))
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"revoked":true`)) {
		t.Fatalf("state = %s", rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/den/", nil)
	req.Header.Set("X-Dev-User", "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"revoked":true`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"revoked_at"`)) {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
}
