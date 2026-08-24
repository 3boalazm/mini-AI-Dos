// Command gateway is the Mini AI-DOS entry point: load configuration,
// build the logger and provider, serve HTTP, shut down cleanly on
// SIGINT/SIGTERM.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	foundationconfig "github.com/ai-dos/foundation/config"
	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/config"
	"github.com/ai-dos/gateway/internal/provider"
	"github.com/ai-dos/gateway/internal/server"
	"github.com/ai-dos/gateway/internal/store"
)

const (
	// shutdownGrace is how long in-flight requests get to finish.
	shutdownGrace = 15 * time.Second
	// dbConnectTimeout bounds the startup connection attempt in database
	// auth mode — fail fast with a clear error, don't hang.
	dbConnectTimeout = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		// Configuration failures land here before the logger exists in a
		// known-good state, so plain stderr is the reliable channel.
		fmt.Fprintf(os.Stderr, "mini-ai-dos: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(foundationconfig.New())
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	env := logging.EnvProduction
	if cfg.Env == "development" {
		env = logging.EnvDevelopment
	}
	log := logging.New(logging.Config{Environment: env, Level: cfg.LogLevel})

	// cfg.AITimeout is each upstream's HTTP client timeout — shorter than
	// the server's per-request backstop (AITimeout + margin) so a
	// provider timeout is the one that normally fires.
	var p provider.Provider
	switch {
	case len(cfg.AIProviders) > 0:
		ups := make([]provider.Upstream, 0, len(cfg.AIProviders))
		names := make([]string, 0, len(cfg.AIProviders))
		for _, pc := range cfg.AIProviders {
			ups = append(ups, provider.Upstream{
				Name:     pc.Name,
				Model:    pc.Model,
				Provider: provider.NewOpenAI(pc.BaseURL, pc.APIKey, cfg.AITimeout, log),
				Timeout:  pc.Timeout,
			})
			names = append(names, pc.Name)
		}
		p = provider.NewFailover(ups, log)
		log.Info("provider configured", "provider", "failover", "chain", names, "timeout", cfg.AITimeout.String())
	case cfg.Provider == config.ProviderOpenAI:
		p = provider.NewOpenAI(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AITimeout, log)
		log.Info("provider configured", "provider", "openai", "base_url", cfg.AIBaseURL, "timeout", cfg.AITimeout.String())
	default:
		p = provider.NewMock()
		log.Info("provider configured", "provider", "mock")
	}

	// Database auth mode: connect and verify readiness up front. Any
	// failure here aborts startup — security configuration never
	// silently degrades to env-key auth.
	var repo store.Repository
	var pg *store.Postgres
	if cfg.AuthMode == config.AuthModeDatabase {
		ctx, cancel := context.WithTimeout(context.Background(), dbConnectTimeout)
		var err error
		pg, err = store.NewPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			cancel()
			return fmt.Errorf("database auth mode: %w", err)
		}
		if err := pg.Ready(ctx); err != nil {
			cancel()
			pg.Close()
			return fmt.Errorf("database auth mode: %w", err)
		}
		cancel()
		defer pg.Close()
		repo = pg
		log.Info("database connected", "auth_mode", "database")
	} else {
		log.Info("auth configured", "auth_mode", "env")
	}

	srv := server.New(cfg, log, p, repo)
	ln, err := srv.Listen()
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", cfg.Port, err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Info("shutdown signal received", "signal", sig.String())
		return srv.Shutdown(shutdownGrace)
	case err := <-errCh:
		return err
	}
}
