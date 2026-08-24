// Package config defines the gateway's own configuration struct,
// loaded through the foundation config.Loader seam — this package
// decides WHAT the gateway needs; the foundation decides HOW values
// are read (real env vs. test map).
package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

// ProviderConfig is one upstream in the failover chain, with its API
// key already resolved from the environment.
type ProviderConfig struct {
	Name    string
	BaseURL string
	APIKey  string
	Model   string
	// Timeout bounds one attempt against this upstream (its entry's
	// timeout_seconds; the global AI_TIMEOUT when unset). A slow or
	// unreachable upstream — a laptop node behind a tunnel — gets a
	// tight bound so its failure costs seconds, not the global limit.
	Timeout time.Duration
}

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
	// AgentWorkspaceDir roots the per-run file workspaces the agent
	// loop writes into (AGENT_WORKSPACE_DIR). Empty falls back to a
	// temp directory chosen by the agent engine.
	AgentWorkspaceDir string
	// AIProviders is the ordered failover chain (AI_PROVIDERS, a JSON
	// array). When non-empty the gateway runs in failover mode: each
	// request tries these upstreams in order until one succeeds, and
	// AI_PROVIDER / AI_API_KEY / AI_BASE_URL are ignored. Empty means
	// single-provider mode (AI_PROVIDER).
	AIProviders []ProviderConfig
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
		AgentWorkspaceDir: l.OptionalString("AGENT_WORKSPACE_DIR", ""),
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

	// Failover mode takes precedence: when AI_PROVIDERS is set, the
	// gateway routes through that chain and the single-provider
	// variables (AI_PROVIDER / AI_API_KEY / AI_BASE_URL) are ignored.
	if raw := strings.TrimSpace(l.OptionalString("AI_PROVIDERS", "")); raw != "" {
		providers, err := parseProviders(l, raw, cfg.AITimeout)
		if err != nil {
			return nil, err
		}
		cfg.AIProviders = providers
		// Callers may omit "model" and there is no single upstream model;
		// a sentinel keeps request validation happy — the failover
		// provider overrides it per upstream anyway.
		if cfg.AIModel == "" {
			cfg.AIModel = "auto"
		}
	} else {
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

// parseProviders decodes the AI_PROVIDERS JSON array and resolves each
// entry's API key. Keys are referenced by env-var name (key_env) so the
// JSON blob itself carries no secrets and can be committed as an
// example; a literal "key" is also accepted for convenience.
func parseProviders(l *foundationconfig.Loader, raw string, defaultTimeout time.Duration) ([]ProviderConfig, error) {
	var entries []struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		KeyEnv  string `json:"key_env"`
		Key     string `json:"key"`
		Model   string `json:"model"`
		// TimeoutSeconds bounds one attempt against this upstream;
		// 0/omitted inherits the global AI_TIMEOUT.
		TimeoutSeconds int `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("AI_PROVIDERS must be a JSON array of provider objects: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("AI_PROVIDERS is an empty array; set AI_PROVIDER instead, or add providers")
	}
	out := make([]ProviderConfig, 0, len(entries))
	for i, e := range entries {
		if e.Name == "" || e.BaseURL == "" || e.Model == "" {
			return nil, fmt.Errorf("AI_PROVIDERS[%d] requires name, base_url, and model", i)
		}
		key := e.Key
		if e.KeyEnv != "" {
			key = l.OptionalString(e.KeyEnv, "")
		}
		if key == "" {
			return nil, fmt.Errorf("AI_PROVIDERS[%d] (%s): API key missing (env %q is unset and no literal key given)", i, e.Name, e.KeyEnv)
		}
		if e.TimeoutSeconds < 0 || e.TimeoutSeconds > 600 {
			return nil, fmt.Errorf("AI_PROVIDERS[%d] (%s): timeout_seconds must be between 1 and 600 (or omitted), got %d", i, e.Name, e.TimeoutSeconds)
		}
		timeout := defaultTimeout
		if e.TimeoutSeconds > 0 {
			timeout = time.Duration(e.TimeoutSeconds) * time.Second
		}
		out = append(out, ProviderConfig{Name: e.Name, BaseURL: e.BaseURL, APIKey: key, Model: e.Model, Timeout: timeout})
	}
	return out, nil
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
