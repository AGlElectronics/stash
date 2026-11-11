package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/AGlElectronics/stash/backend/internal/clientip"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
)

type AuthMode string

const (
	AuthModeOIDC AuthMode = "oidc"
	AuthModeDev  AuthMode = "dev"
)

type Config struct {
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

	// Guest self-service Lost & Found lookup (/claim, /api/public/lostfound).
	// Off by default; enable after the con or when kiosks are ready.
	PublicLostFoundClaimEnabled bool

	// DenInternalURL replaces the host of pasted Den pairing URLs (Docker).
	DenInternalURL string
	// DenMode is DEN_MODE: required keeps the API locked until the module
	// is paired with Den; standalone (the default) does not.
	DenMode denlink.Mode

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
	// TrustedProxyCIDRs lists the proxies whose X-Forwarded-For entries are
	// believed when working out the client IP (TRUSTED_PROXY_CIDRS).
	TrustedProxyCIDRs []netip.Prefix

	ShutdownTimeout time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:                      getEnv("APP_ENV", "development"),
		AppBaseURL:                  getEnv("APP_BASE_URL", "http://localhost:8080"),
		HTTPAddr:                    getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:                 getEnv("DATABASE_URL", ""),
		S3Endpoint:                  getEnv("S3_ENDPOINT", ""),
		S3Bucket:                    getEnv("S3_BUCKET", ""),
		S3AccessKeyID:               getEnv("S3_ACCESS_KEY_ID", ""),
		S3SecretAccessKey:           getEnv("S3_SECRET_ACCESS_KEY", ""),
		S3Region:                    getEnv("S3_REGION", "us-east-1"),
		S3UsePathStyle:              getEnvBool("S3_USE_PATH_STYLE", true),
		AuthMode:                    AuthMode(getEnv("AUTH_MODE", "dev")),
		OIDCIssuerURL:               getEnv("OIDC_ISSUER_URL", ""),
		OIDCClientID:                getEnv("OIDC_CLIENT_ID", ""),
		OIDCAudience:                getEnv("OIDC_AUDIENCE", ""),
		AccessConfigPath:            accessConfigPath(),
		DevDefaultUser:              getEnv("DEV_DEFAULT_USER", "dev-admin"),
		PublicLostFoundClaimEnabled: getEnvBool("PUBLIC_LOSTFOUND_CLAIM_ENABLED", false),
		DenInternalURL:              getEnv("DEN_INTERNAL_URL", ""),
		DenMode:                     denlink.Mode(getEnv("DEN_MODE", string(denlink.ModeStandalone))),
		TelegramBotToken:            strings.TrimSpace(getEnv("TELEGRAM_BOT_TOKEN", "")),
		TelegramAPIURL:              getEnv("TELEGRAM_API_URL", "https://api.telegram.org"),
		SMTPPassword:                getEnv("SMTP_PASSWORD", ""),
		JobsEnabled:                 getEnvBool("JOBS_ENABLED", true),
		CORSAllowedOrigins:          splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		ShutdownTimeout:             getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	trusted, err := clientip.ParseCIDRs(getEnv("TRUSTED_PROXY_CIDRS", clientip.DefaultTrustedProxies))
	if err != nil {
		return nil, fmt.Errorf("invalid config: TRUSTED_PROXY_CIDRS: %w", err)
	}
	cfg.TrustedProxyCIDRs = trusted

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	var errs []string
	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	if err := c.DenMode.Validate(); err != nil {
		errs = append(errs, err.Error())
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
	switch c.AuthMode {
	case AuthModeOIDC:
		if c.OIDCIssuerURL == "" {
			errs = append(errs, "OIDC_ISSUER_URL is required when AUTH_MODE=oidc")
		}
		if c.OIDCClientID == "" {
			errs = append(errs, "OIDC_CLIENT_ID is required when AUTH_MODE=oidc")
		}
		if c.AccessConfigPath == "" {
			errs = append(errs, "ACCESS_CONFIG_PATH is required when AUTH_MODE=oidc (access config TOML file)")
		}
	case AuthModeDev:
		// dev mode: no extra required fields
	default:
		errs = append(errs, fmt.Sprintf("AUTH_MODE must be 'oidc' or 'dev', got %q", c.AuthMode))
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
