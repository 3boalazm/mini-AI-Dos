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
