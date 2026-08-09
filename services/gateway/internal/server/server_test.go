package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/config"
	"github.com/ai-dos/gateway/internal/provider"
)

const testAPIKey = "test-gateway-key"

// newTestServer builds a fully-wired Server (all middleware) around the
// mock provider, returning its handler for httptest use.
func newTestServer(t *testing.T, mutate func(*config.Config)) http.Handler {
	t.Helper()
	cfg := &config.Config{
		Port:              8080,
		APIKey:            testAPIKey,
		Provider:          config.ProviderMock,
		Env:               "development",
		LogLevel:          "error",
		AgentWorkspaceDir: t.TempDir(),
	}
	if mutate != nil {
		mutate(cfg)
	}
	log := logging.New(logging.Config{Environment: logging.EnvDevelopment, Output: io.Discard})
	return New(cfg, log, provider.NewMock(), nil).httpSrv.Handler
}

func doJSON(t *testing.T, h http.Handler, method, path, apiKey, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v — body: %s", err, rec.Body.String())
	}
	return body.Error.Code
}

const validChat = `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

func TestRoot_ServesLandingPage(t *testing.T) {
	h := newTestServer(t, nil)
	rec := doJSON(t, h, http.MethodGet, "/", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("root: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("root Content-Type: got %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "Mini AI-DOS Gateway") {
		t.Error("root page should name the service")
	}
}

func TestChat_ServesChatPage(t *testing.T) {
	h := newTestServer(t, nil)
	rec := doJSON(t, h, http.MethodGet, "/chat", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("chat: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("chat Content-Type: got %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "/v1/chat/completions") {
		t.Error("chat page should call the completions API")
	}
	if !strings.Contains(rec.Body.String(), "statuschip") {
		t.Error("chat page should carry the status-machine chip the UI architecture is built on")
	}
}

func TestRoot_UnknownPathStays404(t *testing.T) {
	h := newTestServer(t, nil)
	rec := doJSON(t, h, http.MethodGet, "/definitely-not-a-route", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path: got %d, want 404", rec.Code)
	}
}

func TestRoot_MethodNotAllowed(t *testing.T) {
	h := newTestServer(t, nil)
	rec := doJSON(t, h, http.MethodPost, "/", "", "{}")
	if rec.Code == http.StatusOK {
		t.Fatal("POST / should not return 200")
	}
	if rec.Header().Get("Allow") != http.MethodGet {
		t.Errorf("Allow header: got %q, want GET", rec.Header().Get("Allow"))
	}
}

func TestHealth_NoAuthRequired(t *testing.T) {
	h := newTestServer(t, nil)
	rec := doJSON(t, h, http.MethodGet, "/health", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("health: got %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body not JSON: %v", err)
	}
	if body["status"] != "ok" || body["provider"] != "mock" {
		t.Errorf("health body wrong: %v", body)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("every response should carry X-Request-Id")
	}
}

func TestChat_MissingKey401(t *testing.T) {
	rec := doJSON(t, newTestServer(t, nil), http.MethodPost, "/v1/chat/completions", "", validChat)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
	if code := errorCode(t, rec); code != "unauthorized" {
		t.Errorf("error code: got %q", code)
	}
}

func TestChat_InvalidKey401(t *testing.T) {
	rec := doJSON(t, newTestServer(t, nil), http.MethodPost, "/v1/chat/completions", "wrong-key", validChat)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestChat_Success(t *testing.T) {
	rec := doJSON(t, newTestServer(t, nil), http.MethodPost, "/v1/chat/completions", testAPIKey, validChat)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q", ct)
	}
	var resp provider.ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp.Object != "chat.completion" || len(resp.Choices) != 1 || resp.Model != "test-model" {
		t.Errorf("normalized shape wrong: %+v", resp)
	}
}

func TestChat_ValidationFailures(t *testing.T) {
	cases := map[string]string{
		"malformed JSON":      `{"model": "x", "messages": [`,
		"no messages":         `{"model":"m","messages":[]}`,
		"bad role":            `{"model":"m","messages":[{"role":"wizard","content":"x"}]}`,
		"empty content":       `{"model":"m","messages":[{"role":"user","content":""}]}`,
		"array content":       `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"x"}]}]}`,
		"temperature high":    `{"model":"m","temperature":3,"messages":[{"role":"user","content":"x"}]}`,
		"max_tokens zero":     `{"model":"m","max_tokens":0,"messages":[{"role":"user","content":"x"}]}`,
		"stream requested":    `{"model":"m","stream":true,"messages":[{"role":"user","content":"x"}]}`,
		"no model no default": `{"messages":[{"role":"user","content":"x"}]}`,
	}
	h := newTestServer(t, nil)
	for name, body := range cases {
		rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", testAPIKey, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400 — body: %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestChat_ModelDefaultFromConfig(t *testing.T) {
	h := newTestServer(t, func(c *config.Config) { c.AIModel = "default-model" })
	rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", testAPIKey,
		`{"messages":[{"role":"user","content":"x"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 with AI_MODEL default — body: %s", rec.Code, rec.Body.String())
	}
	var resp provider.ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp.Model != "default-model" {
		t.Errorf("model: got %q, want config default", resp.Model)
	}
}

func TestChat_WrongContentType415(t *testing.T) {
	h := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChat))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("got %d, want 415", rec.Code)
	}
}

func TestChat_MethodNotAllowed(t *testing.T) {
	rec := doJSON(t, newTestServer(t, nil), http.MethodGet, "/v1/chat/completions", testAPIKey, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d (GET should be rejected)", rec.Code)
	}
}

func TestChat_BodyTooLarge413(t *testing.T) {
	huge := `{"model":"m","messages":[{"role":"user","content":"` + strings.Repeat("a", maxRequestBytes) + `"}]}`
	rec := doJSON(t, newTestServer(t, nil), http.MethodPost, "/v1/chat/completions", testAPIKey, huge)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d, want 413", rec.Code)
	}
}

func TestChat_RateLimit429(t *testing.T) {
	h := newTestServer(t, func(c *config.Config) {
		c.RateLimitEnabled = true
		c.RateLimitRequests = 2
		c.RateLimitWindow = time.Minute
	})
	for i := 0; i < 2; i++ {
		rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", testAPIKey, validChat)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d should pass, got %d", i+1, rec.Code)
		}
		if got, want := rec.Header().Get("X-RateLimit-Remaining"), strconv.Itoa(2-(i+1)); got != want {
			t.Errorf("request %d X-RateLimit-Remaining: got %q, want %q", i+1, got, want)
		}
		if got := rec.Header().Get("X-RateLimit-Limit"); got != "2" {
			t.Errorf("X-RateLimit-Limit: got %q, want \"2\"", got)
		}
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", testAPIKey, validChat)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 should carry Retry-After")
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("429 X-RateLimit-Remaining: got %q, want \"0\"", got)
	}
	if code := errorCode(t, rec); code != "rate_limited" {
		t.Errorf("error code: got %q", code)
	}
}

func TestUnauthenticatedRequestsDoNotConsumeRateLimit(t *testing.T) {
	h := newTestServer(t, func(c *config.Config) {
		c.RateLimitEnabled = true
		c.RateLimitRequests = 1
		c.RateLimitWindow = time.Minute
	})
	// Auth runs before rate limiting: rejected callers can't starve the
	// window for legitimate ones.
	for i := 0; i < 3; i++ {
		doJSON(t, h, http.MethodPost, "/v1/chat/completions", "wrong", validChat)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", testAPIKey, validChat)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated request should still fit the window, got %d", rec.Code)
	}
}

func TestAgentRun_RequiresAuth(t *testing.T) {
	rec := doJSON(t, newTestServer(t, nil), http.MethodPost, "/v1/agent/runs", "", `{"task":"x"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestAgentRun_EmptyTaskRejected(t *testing.T) {
	h := newTestServer(t, func(c *config.Config) { c.AIModel = "agent-model" })
	rec := doJSON(t, h, http.MethodPost, "/v1/agent/runs", testAPIKey, `{"task":"  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 — body: %s", rec.Code, rec.Body.String())
	}
}

func TestAgentRun_UnknownId404(t *testing.T) {
	h := newTestServer(t, func(c *config.Config) { c.AIModel = "agent-model" })
	rec := doJSON(t, h, http.MethodGet, "/v1/agent/runs/run_nope", testAPIKey, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404 — body: %s", rec.Code, rec.Body.String())
	}
}

func TestAgentRun_EndToEndWithMock(t *testing.T) {
	h := newTestServer(t, func(c *config.Config) { c.AIModel = "agent-model" })

	rec := doJSON(t, h, http.MethodPost, "/v1/agent/runs", testAPIKey, `{"task":"build a tiny page"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create: got %d, want 202 — body: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("create body missing run id: %v — %s", err, rec.Body.String())
	}

	// The mock provider answers instantly, so the loop finishes fast;
	// poll like a real client would.
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec = doJSON(t, h, http.MethodGet, "/v1/agent/runs/"+created.ID, testAPIKey, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("poll: got %d — body: %s", rec.Code, rec.Body.String())
		}
		var run struct {
			Status string   `json:"status"`
			Result string   `json:"result"`
			Error  string   `json:"error"`
			Files  []string `json:"files"`
			Steps  []struct{ Status string }
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
			t.Fatalf("poll body: %v", err)
		}
		if run.Status == "completed" {
			if run.Result == "" {
				t.Error("completed run should carry a result")
			}
			// A file the run wrote must be fetchable; an unknown one 404s.
			if len(run.Files) > 0 {
				fr := doJSON(t, h, http.MethodGet, "/v1/agent/runs/"+created.ID+"/files/"+run.Files[0], testAPIKey, "")
				if fr.Code != http.StatusOK {
					t.Errorf("fetch workspace file: got %d, want 200", fr.Code)
				}
				if ct := fr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
					t.Errorf("workspace file should be served as text/plain, got %q", ct)
				}
				if fr.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Error("workspace file must be served with nosniff")
				}
			}
			miss := doJSON(t, h, http.MethodGet, "/v1/agent/runs/"+created.ID+"/files/nope.txt", testAPIKey, "")
			if miss.Code != http.StatusNotFound {
				t.Errorf("missing workspace file: got %d, want 404", miss.Code)
			}
			return
		}
		if run.Status == "failed" {
			t.Fatalf("run failed: %s", run.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("run never completed, last status %s", run.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestErrorBody_IsOpenAIShape(t *testing.T) {
	rec := doJSON(t, newTestServer(t, nil), http.MethodPost, "/v1/chat/completions", "", validChat)
	var body openAIError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not the OpenAI error shape: %s", rec.Body.String())
	}
	if body.Error.Type != "authentication_error" || body.Error.Message == "" {
		t.Errorf("error body: %+v", body)
	}
}
