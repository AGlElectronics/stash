// Package app wires configuration, the database pool and HTTP into a runnable
// server. main.go calls Run.
package app

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/config"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
	"github.com/AGlElectronics/stash/backend/internal/files"
	httpx "github.com/AGlElectronics/stash/backend/internal/http"
	"github.com/AGlElectronics/stash/backend/internal/jobs"
	"github.com/AGlElectronics/stash/backend/internal/mail"
	"github.com/AGlElectronics/stash/backend/internal/service"
	"github.com/AGlElectronics/stash/backend/internal/telegram"
	"github.com/AGlElectronics/stash/backend/internal/tlscert"
)

// Version is set via -ldflags at build time.
var Version = "dev"

// Run boots the server. It blocks until a shutdown signal is received.
func Run(ctx context.Context, logger zerolog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger.Info().
		Str("env", cfg.AppEnv).
		Str("addr", cfg.HTTPAddr).
		Str("auth_mode", string(cfg.AuthMode)).
		Str("version", Version).
		Msg("starting stash backend")

	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pool, err := db.Open(dbCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	migrationCtx, cancelMigrate := context.WithTimeout(ctx, 60*time.Second)
	if err := db.MigrateUp(migrationCtx, pool); err != nil {
		cancelMigrate()
		return fmt.Errorf("run migrations: %w", err)
	}
	cancelMigrate()
	logger.Info().Msg("migrations applied")

	filesCtx, cancelFiles := context.WithTimeout(ctx, 30*time.Second)
	filesClient, err := files.New(filesCtx, cfg)
	cancelFiles()
	if err != nil {
		return fmt.Errorf("init files: %w", err)
	}
	filesSvc := files.NewService(pool, filesClient)
	logger.Info().Str("bucket", filesClient.Bucket()).Msg("object storage ready")

	var oidcVerifier *auth.OIDCVerifier
	if cfg.AuthMode == config.AuthModeOIDC {
		oidcCtx, cancelOIDC := context.WithTimeout(ctx, 30*time.Second)
		oidcVerifier, err = auth.NewOIDCVerifier(oidcCtx, cfg)
		cancelOIDC()
		if err != nil {
			return fmt.Errorf("init oidc verifier: %w", err)
		}
		logger.Info().Str("issuer", cfg.OIDCIssuerURL).Msg("oidc verifier ready")
	}

	// Access config: roles → permissions plus OIDC group → role bindings.
	// The file is the only source of truth (validated as required in OIDC
	// mode by config.Load). Dev setups without a file use the built-in
	// role catalog; dev users are admins.
	var access *auth.AccessConfig
	if cfg.AccessConfigPath != "" {
		access, err = auth.LoadAccessConfig(cfg.AccessConfigPath)
		if err != nil {
			return fmt.Errorf("load access config: %w", err)
		}
		logger.Info().
			Str("path", cfg.AccessConfigPath).
			Int("bindings", len(access.Bindings.All())).
			Strs("roles", access.Roles()).
			Msg("access config loaded")
	} else {
		access = auth.DefaultAccessConfig()
		logger.Info().Msg("no access config file set; using built-in role catalog")
	}

	den := denlink.New(pool, denlink.Config{Slug: "stash", Version: Version, InternalURL: cfg.DenInternalURL, Mode: cfg.DenMode}, logger)
	if err := den.Restore(ctx); err != nil {
		logger.Warn().Err(err).Msg("den link: load failed")
	}
	go den.Run(ctx)

	settings := service.NewSettingsService(pool).WithDen(service.DenSource(den))
	var bot *telegram.Client
	if cfg.TelegramBotToken != "" {
		bot = telegram.New(cfg.TelegramBotToken, cfg.TelegramAPIURL)
	}
	lfBot := service.NewLostFoundTelegram(pool, settings, bot, logger)
	overdue := service.NewOverdueService(pool, settings, mail.SMTP{}, cfg.SMTPPassword, cfg.AppBaseURL, logger)
	if cfg.JobsEnabled {
		go jobs.NewLeader(pool, logger, backgroundJobs(pool, settings, filesSvc, overdue, logger), lfBot.Run).Run(ctx)
	}

	router := httpx.NewRouter(httpx.Deps{
		Den:          den,
		Config:       cfg,
		Pool:         pool,
		Logger:       logger,
		Files:        filesSvc,
		OIDCVerifier: oidcVerifier,
		Access:       access,
		Telegram:     lfBot,
		Overdue:      overdue,
	}, Version)

	server := &nethttp.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if _, err := tlscert.Configure(ctx, server, logger); err != nil {
		return fmt.Errorf("tls: %w", err)
	}

	return runServer(ctx, server, pool, cfg.ShutdownTimeout, logger)
}

// backgroundJobs are run by the leader replica (see internal/jobs).
func backgroundJobs(pool *pgxpool.Pool, settings *service.SettingsService, objects service.ObjectRemover, overdue *service.OverdueService, logger zerolog.Logger) []jobs.Job {
	retention := service.NewRetentionService(pool, settings, objects)
	users := service.NewUserService(pool)
	return []jobs.Job{
		{Name: "lostfound-retention", Every: time.Hour, Run: func(ctx context.Context) error {
			res, err := retention.PurgeLostFound(ctx, time.Now())
			if len(res.Deleted) > 0 {
				logger.Info().Int("reports", len(res.Deleted)).Str("mode", res.Mode).Msg("lost & found retention: reports deleted")
			}
			return err
		}},
		{Name: "user-anonymise", Every: 6 * time.Hour, Run: func(ctx context.Context) error {
			set, err := settings.Get(ctx)
			if err != nil {
				return err
			}
			n, err := users.AnonymiseInactive(ctx, set.UserInactiveDays, time.Now())
			if n > 0 {
				logger.Info().Int("users", n).Msg("inactive users anonymised")
			}
			return err
		}},
		{Name: "overdue-reminders", Every: time.Hour, Run: func(ctx context.Context) error {
			n, err := overdue.SendReminders(ctx, time.Now())
			if n > 0 {
				logger.Info().Int("signouts", n).Msg("overdue reminders sent")
			}
			return err
		}},
	}
}

func runServer(ctx context.Context, server *nethttp.Server, pool *pgxpool.Pool, shutdownTimeout time.Duration, logger zerolog.Logger) error {
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info().Str("addr", server.Addr).Msg("http listening")
		if err := tlscert.ListenAndServe(server); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-signalCtx.Done():
		logger.Info().Msg("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("http shutdown error")
	}
	pool.Close()
	logger.Info().Msg("shutdown complete")
	return nil
}
