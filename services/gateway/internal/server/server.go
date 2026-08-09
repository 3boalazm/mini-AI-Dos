// Package server wires the gateway's HTTP surface: four routes, the
// middleware chain (observability → auth → rate limit), and graceful
// shutdown. SQL never appears here — there is no database layer yet
// (infrastructure-blocked; see the repository README).
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/foundation/util"
	"github.com/ai-dos/gateway/internal/config"
	"github.com/ai-dos/gateway/internal/provider"
	"github.com/ai-dos/gateway/internal/ratelimit"
	"github.com/ai-dos/gateway/internal/store"
)

// Server owns the HTTP server and its dependencies.
type Server struct {
	cfg      *config.Config
	log      *logging.Logger
	provider provider.Provider
	// repo is non-nil exactly when cfg.AuthMode is "database". There is
	// no fallback path: with a repo present, the env key is never
	// consulted; without one, the database is never consulted.
	repo    store.Repository
	limiter *ratelimit.Limiter
	httpSrv *http.Server
}

// New assembles a Server from already-constructed dependencies.
// Provider and repository construction live in cmd/gateway, not here —
// the server does not know how they are configured, only how to call
// them. repo must be nil in env auth mode and non-nil in database
// mode; New enforces the pairing rather than trusting the caller.
func New(cfg *config.Config, log *logging.Logger, p provider.Provider, repo store.Repository) *Server {
	if (cfg.AuthMode == config.AuthModeDatabase) != (repo != nil) {
		// A mismatch is a programming error in the wiring, not a runtime
		// condition to tolerate — failing loudly here is what prevents a
		// silent fallback from database auth to env auth.
		panic("server.New: repo must be provided if and only if AuthMode is database")
	}
	s := &Server{cfg: cfg, log: log, provider: p, repo: repo}

	if cfg.RateLimitEnabled {
		s.limiter = ratelimit.New(cfg.RateLimitRequests, cfg.RateLimitWindow, util.RealClock{})
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(s.handleRoot))
	mux.Handle("/chat", http.HandlerFunc(s.handleChat))
	mux.Handle("/health", http.HandlerFunc(s.handleHealth))
	mux.Handle("/v1/chat/completions",
		s.withAuth(s.withRateLimit(http.HandlerFunc(s.handleChatCompletions))))

	s.httpSrv = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           s.withObservability(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      2 * completionTimeout,
		IdleTimeout:       120 * time.Second,
	}

	return s
}

// completionContext derives the per-completion backstop timeout.
func (s *Server) completionContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), completionTimeout)
}

// Serve accepts connections on ln until Shutdown is called. It returns
// nil after a clean shutdown, mirroring http.Server.Serve semantics
// minus the awkward ErrServerClosed special case.
func (s *Server) Serve(ln net.Listener) error {
	s.log.Info("mini ai-dos started",
		"addr", ln.Addr().String(),
		"provider", s.provider.Name(),
		"rate_limit_enabled", s.cfg.RateLimitEnabled,
	)
	if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Listen opens the configured port. Split from Serve so tests (and
// main) know the bound address before serving begins.
func (s *Server) Listen() (net.Listener, error) {
	return net.Listen("tcp", s.httpSrv.Addr)
}

// Shutdown performs the graceful sequence: stop accepting connections,
// let in-flight requests finish within grace, then return. The caller
// (main) closes anything the server does not own — providers hold no
// closable resources today, and there is no database connection yet.
func (s *Server) Shutdown(grace time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	err := s.httpSrv.Shutdown(ctx)
	if err != nil {
		s.log.Warn("shutdown did not complete cleanly", "error", err.Error())
		return err
	}
	s.log.Info("mini ai-dos stopped cleanly")
	return nil
}
