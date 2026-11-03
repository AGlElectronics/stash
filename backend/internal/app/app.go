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
	"github.com/AGlElectronics/stash/backend/internal/files"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
	httpx "github.com/AGlElectronics/stash/backend/internal/http"
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

	den := denlink.New(pool, denlink.Config{Slug: "stash", Version: Version, InternalURL: cfg.DenInternalURL}, logger)
	go den.Run(ctx)

	router := httpx.NewRouter(httpx.Deps{
		Den:          den,
		Config:       cfg,
		Pool:         pool,
		Logger:       logger,
		Files:        filesSvc,
		OIDCVerifier: oidcVerifier,
		Access:       access,
	}, Version)

	server := &nethttp.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return runServer(ctx, server, pool, cfg.ShutdownTimeout, logger)
}

func runServer(ctx context.Context, server *nethttp.Server, pool *pgxpool.Pool, shutdownTimeout time.Duration, logger zerolog.Logger) error {
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info().Str("addr", server.Addr).Msg("http listening")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
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
