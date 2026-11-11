package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/AGlElectronics/stash/backend/internal/authbase"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
	"github.com/AGlElectronics/stash/backend/internal/groupmatch"
)

// AuthMode selects how requests are authenticated (authbase). AuthModeDev
// trusts the X-Dev-User header and is only allowed with
// APP_ENV=development; AuthModeOIDC verifies Keycloak access tokens.
type AuthMode = authbase.AuthMode

const (
	AuthModeOIDC = authbase.AuthOIDC
	AuthModeDev  = authbase.AuthDev
)

// DefaultAudience is OIDC_AUDIENCE when unset: the audience Keycloak adds to
// the access tokens of Stash's web app (token contract).
const DefaultAudience = "stash-api"

// MinHRServiceTokenLength is the shortest HR_SERVICE_TOKEN accepted.
const MinHRServiceTokenLength = 32

type Config struct {
	// AppEnv is APP_ENV, normalised; production unless set to development.
	AppEnv      string
	AppBaseURL  string
	HTTPAddr    string
	DatabaseURL string

	S3Endpoint        string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3Region          string
	S3UsePathStyle    bool

	AuthMode      AuthMode
	OIDCIssuerURL string
	OIDCClientID  string
	OIDCAudience  string
	// Path to the access config TOML (roles, permissions, OIDC bindings).
	// ACCESS_CONFIG_PATH is preferred; OIDC_BINDINGS_PATH is kept for compat.
	AccessConfigPath string
	DevDefaultUser   string
	// GroupMatch is OIDC_GROUPS_CLAIM, OIDC_GROUP_MATCH and OIDC_GROUP_PREFIX.
	GroupMatch groupmatch.Config
	// HRServiceToken lets HR read GET /api/role-catalog without a sign-in
	// (HR_SERVICE_TOKEN). Empty: only signed-in staff may read it.
	HRServiceToken string

	// Guest self-service Lost & Found lookup (/claim, /api/public/lostfound).
	// Off by default; enable after the con or when kiosks are ready.
	PublicLostFoundClaimEnabled bool

	// Den is the pairing configuration (DEN_MODE, DEN_INTERNAL_URL,
	// DEN_ALLOWED_HOSTS, DEN_LINK_ENCRYPTION_KEY[_PREVIOUS], APP_ENV).
	Den denlink.Config

	// TelegramBotToken is the Lost & Found bot's token (a secret). Empty
	// means no Telegram, whatever the setting says.
	TelegramBotToken string
	// TelegramAPIURL is the Bot API server (tests point it elsewhere).
	TelegramAPIURL string
	// SMTPPassword logs in to the SMTP server from the settings (a secret).
	SMTPPassword string
	// JobsEnabled runs background jobs (retention, reminders, Telegram) on
	// the replica that wins the leader lock. Off only for debugging.
	JobsEnabled bool

	CORSAllowedOrigins []string
	// TrustedProxies are the proxies whose X-Forwarded-For entries are
	// believed when working out the client IP (TRUSTED_PROXY_CIDRS).
	TrustedProxies authbase.TrustedProxies

	ShutdownTimeout time.Duration
}

// Auth is the authbase view of AUTH_MODE, APP_ENV and TRUSTED_PROXY_CIDRS.
func (c *Config) Auth() authbase.Config {
	return authbase.Config{AppEnv: c.AppEnv, AuthMode: c.AuthMode, TrustedProxies: c.TrustedProxies}
}

// Audience is the audience access tokens must carry.
func (c *Config) Audience() string {
	if c.OIDCAudience != "" {
		return c.OIDCAudience
	}
	return DefaultAudience
}

// DenConfig is Den with the slug and version filled in, and APP_ENV taken
// from the process when Den does not set its own.
func (c *Config) DenConfig(version string) denlink.Config {
	d := c.Den
	d.Slug, d.Version = "stash", version
	if d.AppEnv == "" {
		d.AppEnv = c.AppEnv
	}
	return d
}

// Load reads the environment (plus an optional .env file) and returns a
// validated Config. Every problem is reported at once.
func Load() (*Config, error) {
	_ = godotenv.Load()

	var errs []string
	auth, err := authbase.FromEnv(os.Getenv)
	if err != nil {
		errs = append(errs, err.Error())
	}
	groups, err := groupmatch.FromEnv(os.Getenv)
	if err != nil {
		errs = append(errs, err.Error())
	}
	den, err := denlink.FromEnv("stash", "", os.Getenv)
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg := &Config{
		AppEnv:                      auth.AppEnv,
		AppBaseURL:                  getEnv("APP_BASE_URL", "http://localhost:8080"),
		HTTPAddr:                    getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:                 getEnv("DATABASE_URL", ""),
		S3Endpoint:                  getEnv("S3_ENDPOINT", ""),
		S3Bucket:                    getEnv("S3_BUCKET", ""),
		S3AccessKeyID:               getEnv("S3_ACCESS_KEY_ID", ""),
		S3SecretAccessKey:           getEnv("S3_SECRET_ACCESS_KEY", ""),
		S3Region:                    getEnv("S3_REGION", "us-east-1"),
		S3UsePathStyle:              getEnvBool("S3_USE_PATH_STYLE", true),
		AuthMode:                    auth.AuthMode,
		OIDCIssuerURL:               getEnv("OIDC_ISSUER_URL", ""),
		OIDCClientID:                getEnv("OIDC_CLIENT_ID", ""),
		OIDCAudience:                getEnv("OIDC_AUDIENCE", DefaultAudience),
		AccessConfigPath:            accessConfigPath(),
		DevDefaultUser:              getEnv("DEV_DEFAULT_USER", "dev-admin"),
		GroupMatch:                  groups,
		HRServiceToken:              strings.TrimSpace(getEnv("HR_SERVICE_TOKEN", "")),
		PublicLostFoundClaimEnabled: getEnvBool("PUBLIC_LOSTFOUND_CLAIM_ENABLED", false),
		Den:                         den,
		TelegramBotToken:            strings.TrimSpace(getEnv("TELEGRAM_BOT_TOKEN", "")),
		TelegramAPIURL:              getEnv("TELEGRAM_API_URL", "https://api.telegram.org"),
		SMTPPassword:                getEnv("SMTP_PASSWORD", ""),
		JobsEnabled:                 getEnvBool("JOBS_ENABLED", true),
		CORSAllowedOrigins:          splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		TrustedProxies:              auth.TrustedProxies,
		ShutdownTimeout:             getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	if err := cfg.validate(errs); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate(errs []string) error {
	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	if c.S3Endpoint == "" {
		errs = append(errs, "S3_ENDPOINT is required")
	}
	if c.S3Bucket == "" {
		errs = append(errs, "S3_BUCKET is required")
	}
	if c.S3AccessKeyID == "" {
		errs = append(errs, "S3_ACCESS_KEY_ID is required")
	}
	if c.S3SecretAccessKey == "" {
		errs = append(errs, "S3_SECRET_ACCESS_KEY is required")
	}
	if c.HRServiceToken != "" && len(c.HRServiceToken) < MinHRServiceTokenLength {
		errs = append(errs, fmt.Sprintf("HR_SERVICE_TOKEN must be at least %d characters (generate one with: openssl rand -base64 32)", MinHRServiceTokenLength))
	}
	if c.AuthMode == AuthModeOIDC {
		if c.OIDCIssuerURL == "" {
			errs = append(errs, "OIDC_ISSUER_URL is required when AUTH_MODE=oidc")
		}
		if c.OIDCClientID == "" {
			errs = append(errs, "OIDC_CLIENT_ID is required when AUTH_MODE=oidc")
		}
		if c.AccessConfigPath == "" {
			errs = append(errs, "ACCESS_CONFIG_PATH is required when AUTH_MODE=oidc (access config TOML file)")
		}
	}
	if len(errs) > 0 {
		return errors.New("invalid config: " + strings.Join(errs, "; "))
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// accessConfigPath resolves ACCESS_CONFIG_PATH, falling back to the legacy
// OIDC_BINDINGS_PATH name used before the TOML migration.
func accessConfigPath() string {
	if p := getEnv("ACCESS_CONFIG_PATH", ""); p != "" {
		return p
	}
	return getEnv("OIDC_BINDINGS_PATH", "")
}
