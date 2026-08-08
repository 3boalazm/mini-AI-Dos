// Command gateway is the Mini AI-DOS entry point: load configuration,
// build the logger and provider, serve HTTP, shut down cleanly on
// SIGINT/SIGTERM.
package main

import (
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
)

const (
	// providerTimeout is the upstream HTTP client timeout — shorter than
	// the server's per-request backstop so the provider timeout is the
	// one that normally fires.
	providerTimeout = 60 * time.Second
	// shutdownGrace is how long in-flight requests get to finish.
	shutdownGrace = 15 * time.Second
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

	var p provider.Provider
	switch cfg.Provider {
	case config.ProviderOpenAI:
		p = provider.NewOpenAI(cfg.AIBaseURL, cfg.AIAPIKey, providerTimeout, log)
		log.Info("provider configured", "provider", "openai", "base_url", cfg.AIBaseURL)
	default:
		p = provider.NewMock()
		log.Info("provider configured", "provider", "mock")
	}

	srv := server.New(cfg, log, p)
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
