package config

import (
	"strings"
	"testing"
)

// TestSafeDefaults: without AUTH_MODE and APP_ENV the backend runs OIDC in
// production; dev sign-in, a missing Den key and a short HR token are
// refused (authbase, denlink).
func TestSafeDefaults(t *testing.T) {
	clear := func(t *testing.T) {
		for _, k := range []string{"AUTH_MODE", "APP_ENV", "OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_AUDIENCE",
			"ACCESS_CONFIG_PATH", "OIDC_BINDINGS_PATH", "DEN_LINK_ENCRYPTION_KEY", "DEN_LINK_ENCRYPTION_KEY_PREVIOUS",
			"DEN_MODE", "DEN_INTERNAL_URL", "DEN_ALLOWED_HOSTS", "OIDC_GROUPS_CLAIM", "OIDC_GROUP_MATCH",
			"OIDC_GROUP_PREFIX", "HR_SERVICE_TOKEN", "TRUSTED_PROXY_CIDRS"} {
			t.Setenv(k, "")
		}
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("S3_ENDPOINT", "http://s3")
		t.Setenv("S3_BUCKET", "stash")
		t.Setenv("S3_ACCESS_KEY_ID", "k")
		t.Setenv("S3_SECRET_ACCESS_KEY", "s")
	}
	oidc := func(t *testing.T) {
		t.Setenv("OIDC_ISSUER_URL", "https://auth.example.org/realms/c")
		t.Setenv("OIDC_CLIENT_ID", "stash-spa")
		t.Setenv("ACCESS_CONFIG_PATH", "/etc/stash/access-config.toml")
	}
	key := "ZGV2LW9ubHktc3Rhc2gtZGVuLWxpbmsta2V5LTAwMDA=" // 32 bytes, dev only

	t.Run("production oidc", func(t *testing.T) {
		clear(t)
		oidc(t)
		t.Setenv("DEN_LINK_ENCRYPTION_KEY", key)
		t.Setenv("OIDC_GROUP_MATCH", "last")
		t.Setenv("OIDC_GROUP_PREFIX", "/con2026")
		t.Setenv("HR_SERVICE_TOKEN", strings.Repeat("h", 32))
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AppEnv != "production" || cfg.AuthMode != AuthModeOIDC || cfg.Audience() != "stash-api" {
			t.Fatalf("defaults: env %q, mode %q, audience %q", cfg.AppEnv, cfg.AuthMode, cfg.Audience())
		}
		if cfg.GroupMatch.ClaimName() != "groups" || cfg.GroupMatch.Mode != "last" || cfg.GroupMatch.Prefix != "/con2026" {
			t.Fatalf("group match = %+v", cfg.GroupMatch)
		}
		if d := cfg.DenConfig("1.0.0"); d.Slug != "stash" || d.Version != "1.0.0" || d.EncryptionKey != key {
			t.Fatalf("den config = %+v", d)
		}
		if len(cfg.TrustedProxies) == 0 || cfg.Auth().DevAuth() {
			t.Fatalf("auth = %+v", cfg.Auth())
		}
	})
	t.Run("dev refused in production", func(t *testing.T) {
		clear(t)
		t.Setenv("AUTH_MODE", "dev")
		t.Setenv("DEN_LINK_ENCRYPTION_KEY", key)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AUTH_MODE=dev") {
			t.Fatalf("want the AUTH_MODE=dev refusal, got %v", err)
		}
	})
	t.Run("dev allowed in development without a Den key", func(t *testing.T) {
		clear(t)
		t.Setenv("AUTH_MODE", "dev")
		t.Setenv("APP_ENV", "Development")
		cfg, err := Load()
		if err != nil || !cfg.Auth().DevAuth() {
			t.Fatalf("cfg %+v, err %v", cfg, err)
		}
	})
	t.Run("den key required in production", func(t *testing.T) {
		clear(t)
		oidc(t)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DEN_LINK_ENCRYPTION_KEY") {
			t.Fatalf("want the DEN_LINK_ENCRYPTION_KEY refusal, got %v", err)
		}
	})
	t.Run("short HR token", func(t *testing.T) {
		clear(t)
		oidc(t)
		t.Setenv("DEN_LINK_ENCRYPTION_KEY", key)
		t.Setenv("HR_SERVICE_TOKEN", "too-short")
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "HR_SERVICE_TOKEN") {
			t.Fatalf("want the HR_SERVICE_TOKEN error, got %v", err)
		}
	})
	t.Run("bad group match and proxies", func(t *testing.T) {
		clear(t)
		t.Setenv("APP_ENV", "development")
		t.Setenv("AUTH_MODE", "dev")
		t.Setenv("OIDC_GROUP_MATCH", "fuzzy")
		t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,not-a-cidr")
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "OIDC_GROUP_MATCH") || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
			t.Fatalf("want OIDC_GROUP_MATCH and TRUSTED_PROXY_CIDRS errors, got %v", err)
		}
	})
}
