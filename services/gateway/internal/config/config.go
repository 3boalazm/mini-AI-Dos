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

// Config is everything the gateway reads from the environment.
// Anything not listed here is not configuration the gateway uses.
type Config struct {
	// Port the HTTP server listens on (GATEWAY_PORT, default 8080).
	Port int
	// APIKey callers must present as a Bearer token
	// (MINI_AI_DOS_API_KEY, required).
	APIKey string
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
	required, err := l.RequireString("MINI_AI_DOS_API_KEY")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:              l.OptionalInt("GATEWAY_PORT", 8080),
		APIKey:            required["MINI_AI_DOS_API_KEY"],
		Provider:          l.OptionalString("AI_PROVIDER", ProviderMock),
		AIBaseURL:         l.OptionalString("AI_BASE_URL", "https://api.openai.com/v1"),
		AIModel:           l.OptionalString("AI_MODEL", ""),
		Env:               l.OptionalString("APP_ENV", "development"),
		LogLevel:          l.OptionalString("LOG_LEVEL", "info"),
		RateLimitEnabled:  parseBool(l.OptionalString("RATE_LIMIT_ENABLED", "false")),
		RateLimitRequests: l.OptionalInt("RATE_LIMIT_REQUESTS", 60),
		RateLimitWindow:   time.Duration(l.OptionalInt("RATE_LIMIT_WINDOW", 60)) * time.Second,
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
