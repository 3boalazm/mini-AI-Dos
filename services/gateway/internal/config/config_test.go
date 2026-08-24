package config

import (
	"strings"
	"testing"
	"time"

	foundationconfig "github.com/ai-dos/foundation/config"
)

func load(t *testing.T, env map[string]string) (*Config, error) {
	t.Helper()
	return Load(foundationconfig.NewFromMap(env))
}

func TestLoad_MinimalDefaults(t *testing.T) {
	cfg, err := load(t, map[string]string{"MINI_AI_DOS_API_KEY": "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port default: got %d, want 8080", cfg.Port)
	}
	if cfg.Provider != ProviderMock {
		t.Errorf("Provider default: got %q, want mock", cfg.Provider)
	}
	if cfg.RateLimitEnabled {
		t.Error("rate limiting should default to disabled")
	}
	if cfg.Env != "development" || cfg.LogLevel != "info" {
		t.Errorf("env/log defaults wrong: %q / %q", cfg.Env, cfg.LogLevel)
	}
	if cfg.AITimeout != 120*time.Second {
		t.Errorf("AITimeout default: got %v, want 120s", cfg.AITimeout)
	}
}

func TestLoad_FailoverProviders(t *testing.T) {
	cfg, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"GEMINI_API_KEY":      "gem-key",
		"GROQ_API_KEY":        "groq-key",
		"AI_PROVIDERS": `[
			{"name":"gemini","base_url":"https://g/v1","key_env":"GEMINI_API_KEY","model":"gemini-3.6-flash"},
			{"name":"groq","base_url":"https://q/v1","key_env":"GROQ_API_KEY","model":"gpt-oss-120b"}
		]`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.AIProviders) != 2 {
		t.Fatalf("got %d providers, want 2", len(cfg.AIProviders))
	}
	if cfg.AIProviders[0].APIKey != "gem-key" || cfg.AIProviders[1].APIKey != "groq-key" {
		t.Errorf("keys not resolved from env: %+v", cfg.AIProviders)
	}
	if cfg.AIProviders[0].Model != "gemini-3.6-flash" {
		t.Errorf("model wrong: %q", cfg.AIProviders[0].Model)
	}
	// Failover mode fills a sentinel model so request validation passes.
	if cfg.AIModel != "auto" {
		t.Errorf("failover mode should default AIModel to 'auto', got %q", cfg.AIModel)
	}
	// AI_API_KEY is NOT required in failover mode.
	if cfg.AIAPIKey != "" {
		t.Errorf("AIAPIKey should be unused in failover mode, got %q", cfg.AIAPIKey)
	}
}

func TestLoad_FailoverPerProviderTimeout(t *testing.T) {
	cfg, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"GEMINI_API_KEY":      "gem-key",
		"AI_TIMEOUT":          "90",
		"AI_PROVIDERS": `[
			{"name":"tunnel-qwen","base_url":"https://t/v1","key":"node-key","model":"qwen2.5:3b","timeout_seconds":8},
			{"name":"gemini","base_url":"https://g/v1","key_env":"GEMINI_API_KEY","model":"gemini-3.6-flash"}
		]`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.AIProviders[0].Timeout; got != 8*time.Second {
		t.Errorf("explicit timeout_seconds: got %v, want 8s", got)
	}
	// Unset inherits the global AI_TIMEOUT.
	if got := cfg.AIProviders[1].Timeout; got != 90*time.Second {
		t.Errorf("default timeout should inherit AI_TIMEOUT: got %v, want 90s", got)
	}
}

func TestLoad_FailoverTimeoutRejectsOutOfRange(t *testing.T) {
	_, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_PROVIDERS":        `[{"name":"x","base_url":"https://x/v1","key":"k","model":"m","timeout_seconds":9999}]`,
	})
	if err == nil {
		t.Fatal("expected error for timeout_seconds out of range")
	}
}

func TestLoad_FailoverValidation(t *testing.T) {
	// Missing key_env value.
	if _, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_PROVIDERS":        `[{"name":"gemini","base_url":"https://g/v1","key_env":"NOPE","model":"m"}]`,
	}); err == nil {
		t.Error("provider with an unset key_env should fail")
	}
	// Missing required field (model).
	if _, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"GEMINI_API_KEY":      "x",
		"AI_PROVIDERS":        `[{"name":"gemini","base_url":"https://g/v1","key_env":"GEMINI_API_KEY"}]`,
	}); err == nil {
		t.Error("provider missing model should fail")
	}
	// Malformed JSON.
	if _, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_PROVIDERS":        `not json`,
	}); err == nil {
		t.Error("malformed AI_PROVIDERS should fail")
	}
	// A literal key (no key_env) is accepted.
	cfg, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_PROVIDERS":        `[{"name":"x","base_url":"https://x/v1","key":"literal","model":"m"}]`,
	})
	if err != nil || len(cfg.AIProviders) != 1 || cfg.AIProviders[0].APIKey != "literal" {
		t.Errorf("literal key should be accepted: err=%v cfg=%+v", err, cfg.AIProviders)
	}
}

func TestLoad_AITimeout(t *testing.T) {
	cfg, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_TIMEOUT":          "180",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AITimeout != 180*time.Second {
		t.Errorf("AITimeout: got %v, want 180s", cfg.AITimeout)
	}

	for name, v := range map[string]string{"zero": "0", "negative": "-5", "too large": "601"} {
		if _, err := load(t, map[string]string{"MINI_AI_DOS_API_KEY": "k", "AI_TIMEOUT": v}); err == nil {
			t.Errorf("%s AI_TIMEOUT (%s) should fail validation", name, v)
		}
	}
}

func TestLoad_MissingGatewayKeyFails(t *testing.T) {
	_, err := load(t, map[string]string{})
	if err == nil {
		t.Fatal("expected error when MINI_AI_DOS_API_KEY is missing")
	}
	if !strings.Contains(err.Error(), "MINI_AI_DOS_API_KEY") {
		t.Errorf("error should name the missing variable, got: %v", err)
	}
}

func TestLoad_OpenAIRequiresUpstreamKey(t *testing.T) {
	_, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_PROVIDER":         "openai",
	})
	if err == nil {
		t.Fatal("expected error: openai provider without AI_API_KEY")
	}
	if !strings.Contains(err.Error(), "AI_API_KEY") {
		t.Errorf("error should name AI_API_KEY, got: %v", err)
	}

	cfg, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_PROVIDER":         "openai",
		"AI_API_KEY":          "upstream",
	})
	if err != nil {
		t.Fatalf("unexpected error with AI_API_KEY set: %v", err)
	}
	if cfg.AIAPIKey != "upstream" {
		t.Errorf("AIAPIKey not carried through")
	}
	if cfg.AIBaseURL != "https://api.openai.com/v1" {
		t.Errorf("AIBaseURL default wrong: %q", cfg.AIBaseURL)
	}
}

func TestLoad_DatabaseModeRequiresDatabaseURL(t *testing.T) {
	_, err := load(t, map[string]string{"API_KEY_AUTH_MODE": "database"})
	if err == nil {
		t.Fatal("expected error: database mode without DATABASE_URL")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error should name DATABASE_URL, got: %v", err)
	}
}

func TestLoad_DatabaseModeDoesNotRequireEnvKey(t *testing.T) {
	cfg, err := load(t, map[string]string{
		"API_KEY_AUTH_MODE": "database",
		"DATABASE_URL":      "postgres://u:p@localhost:5432/db",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthMode != AuthModeDatabase || cfg.DatabaseURL == "" {
		t.Errorf("database mode not configured: %+v", cfg)
	}
	if cfg.APIKey != "" {
		t.Errorf("env key must not be loaded in database mode, got %q", cfg.APIKey)
	}
}

func TestLoad_UnknownAuthModeRejected(t *testing.T) {
	_, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"API_KEY_AUTH_MODE":   "keycloak",
	})
	if err == nil {
		t.Fatal("expected error for unknown auth mode")
	}
}

func TestLoad_UnknownProviderRejected(t *testing.T) {
	_, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"AI_PROVIDER":         "anthropic-carrier-pigeon",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestLoad_RateLimitConfig(t *testing.T) {
	cfg, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"RATE_LIMIT_ENABLED":  "true",
		"RATE_LIMIT_REQUESTS": "10",
		"RATE_LIMIT_WINDOW":   "30",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.RateLimitEnabled || cfg.RateLimitRequests != 10 || cfg.RateLimitWindow != 30*time.Second {
		t.Errorf("rate limit config not parsed: %+v", cfg)
	}
}

func TestLoad_InvalidPortRejected(t *testing.T) {
	_, err := load(t, map[string]string{
		"MINI_AI_DOS_API_KEY": "k",
		"GATEWAY_PORT":        "70000",
	})
	if err == nil {
		t.Fatal("expected error for out-of-range port")
	}
}
