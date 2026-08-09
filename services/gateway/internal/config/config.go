// Package config defines the gateway's own configuration struct,
// loaded through the foundation config.Loader seam — this package
// decides WHAT the gateway needs; the foundation decides HOW values
// are read (real env vs. test map).
package config

import (
	"fmt"
	"strconv"
	"time"

	foundationconfig "github.com/ai-dos/foundation/config"
)

// Provider names accepted in AI_PROVIDER.
const (
	ProviderMock   = "mock"
	ProviderOpenAI = "openai"
)

// Auth modes accepted in API_KEY_AUTH_MODE.
const (
	// AuthModeEnv authenticates against the single MINI_AI_DOS_API_KEY
	// environment variable — zero-database development mode.
	AuthModeEnv = "env"
	// AuthModeDatabase authenticates against hashed keys in PostgreSQL.
	AuthModeDatabase = "database"
)

// Config is everything the gateway reads from the environment.
// Anything not listed here is not configuration the gateway uses.
type Config struct {
	// Port the HTTP server listens on (GATEWAY_PORT, default 8080).
	Port int
	// AuthMode selects where API keys are verified: "env" (default) or
	// "database". There is no fallback between them — a misconfigured
	// database mode fails at startup, never silently degrades to env.
	AuthMode string
	// APIKey callers must present as a Bearer token
	// (MINI_AI_DOS_API_KEY — required in env auth mode, unused in
	// database mode).
	APIKey string
	// DatabaseURL is the PostgreSQL connection string (DATABASE_URL —
	// required in database auth mode, unused in env mode).
	DatabaseURL string
	// Provider selects the completion backend (AI_PROVIDER,
	// "mock" or "openai", default "mock").
	Provider string
	// AIAPIKey authenticates the gateway to the upstream provider
	// (AI_API_KEY, required only when Provider is "openai").
	AIAPIKey string
	// AIBaseURL is the upstream API root (AI_BASE_URL, default
	// https://api.openai.com/v1).
	AIBaseURL string
	// AIModel is the default model used when a request omits "model"
	// (AI_MODEL, optional — with no default, requests must name one).
	AIModel string
	// AITimeout bounds one upstream completion call (AI_TIMEOUT,
	// seconds, default 120). Long generations on free-tier providers —
	// a full HTML page from a thinking model — routinely pass 60s, so
	// the default leans generous; deployments tune it down when their
	// traffic is short-form.
	AITimeout time.Duration
	// Env selects log format: "development" (text) or anything else
	// (JSON). APP_ENV, default "development".
	Env string
	// LogLevel is debug|info|warn|error (LOG_LEVEL, default "info").
	LogLevel string
	// Rate limiting for POST /v1/chat/completions.
	RateLimitEnabled  bool
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

// Load reads and validates gateway configuration. Every missing
// required variable is reported at once via MissingEnvError — the
// foundation loader's contract.
func Load(l *foundationconfig.Loader) (*Config, error) {
	cfg := &Config{
		Port:              l.OptionalInt("GATEWAY_PORT", 8080),
		AuthMode:          l.OptionalString("API_KEY_AUTH_MODE", AuthModeEnv),
		Provider:          l.OptionalString("AI_PROVIDER", ProviderMock),
		AIBaseURL:         l.OptionalString("AI_BASE_URL", "https://api.openai.com/v1"),
		AIModel:           l.OptionalString("AI_MODEL", ""),
		AITimeout:         time.Duration(l.OptionalInt("AI_TIMEOUT", 120)) * time.Second,
		Env:               l.OptionalString("APP_ENV", "development"),
		LogLevel:          l.OptionalString("LOG_LEVEL", "info"),
		RateLimitEnabled:  parseBool(l.OptionalString("RATE_LIMIT_ENABLED", "false")),
		RateLimitRequests: l.OptionalInt("RATE_LIMIT_REQUESTS", 60),
		RateLimitWindow:   time.Duration(l.OptionalInt("RATE_LIMIT_WINDOW", 60)) * time.Second,
	}

	switch cfg.AuthMode {
	case AuthModeEnv:
		required, err := l.RequireString("MINI_AI_DOS_API_KEY")
		if err != nil {
			return nil, err
		}
		cfg.APIKey = required["MINI_AI_DOS_API_KEY"]
	case AuthModeDatabase:
		required, err := l.RequireString("DATABASE_URL")
		if err != nil {
			return nil, fmt.Errorf("API_KEY_AUTH_MODE=database requires DATABASE_URL: %w", err)
		}
		cfg.DatabaseURL = required["DATABASE_URL"]
	default:
		return nil, fmt.Errorf("API_KEY_AUTH_MODE must be %q or %q, got %q", AuthModeEnv, AuthModeDatabase, cfg.AuthMode)
	}

	if cfg.Provider != ProviderMock && cfg.Provider != ProviderOpenAI {
		return nil, fmt.Errorf("AI_PROVIDER must be %q or %q, got %q", ProviderMock, ProviderOpenAI, cfg.Provider)
	}

	if cfg.Provider == ProviderOpenAI {
		upstream, err := l.RequireString("AI_API_KEY")
		if err != nil {
			return nil, fmt.Errorf("AI_PROVIDER=openai requires AI_API_KEY: %w", err)
		}
		cfg.AIAPIKey = upstream["AI_API_KEY"]
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("GATEWAY_PORT must be a valid TCP port, got %d", cfg.Port)
	}
	if secs := int(cfg.AITimeout / time.Second); secs < 1 || secs > 600 {
		return nil, fmt.Errorf("AI_TIMEOUT (seconds) must be between 1 and 600, got %d", secs)
	}
	if cfg.RateLimitEnabled && cfg.RateLimitRequests <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_REQUESTS must be positive when rate limiting is enabled, got %d", cfg.RateLimitRequests)
	}
	if cfg.RateLimitEnabled && cfg.RateLimitWindow <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_WINDOW (seconds) must be positive when rate limiting is enabled")
	}

	return cfg, nil
}

// parseBool treats anything unparseable as false — same fail-safe
// stance as the foundation loader's OptionalInt.
func parseBool(v string) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}
