package server

// The end-to-end flow the Phase 10 mandate specifies, against a real
// listening TCP server (not httptest.Recorder): start → health → 401
// without key → 401 with bad key → successful completion through the
// mock provider → response shape verified → logs carry request
// metadata but no secrets → graceful shutdown with an in-flight
// request completing.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/config"
	"github.com/ai-dos/gateway/internal/provider"
)

// httpResult is a fully-consumed HTTP exchange: the response body is
// read and closed before this is returned, so no test path can leak a
// connection.
type httpResult struct {
	status int
	header http.Header
	body   []byte
}

func doRequest(t *testing.T, client *http.Client, method, url, apiKey, body string) (httpResult, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return httpResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResult{}, err
	}
	return httpResult{status: resp.StatusCode, header: resp.Header, body: raw}, nil
}

// syncBuffer makes bytes.Buffer safe for the concurrent writes a live
// server's logger performs.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

const e2eKey = "e2e-secret-gateway-key"

func TestE2E_FullFlow(t *testing.T) {
	logs := &syncBuffer{}
	cfg := &config.Config{
		Port:     0, // OS-assigned free port
		APIKey:   e2eKey,
		Provider: config.ProviderMock,
		Env:      "production", // JSON logs, like a real deployment
		LogLevel: "info",
	}
	log := logging.New(logging.Config{Environment: logging.EnvProduction, Level: "info", Output: logs})
	srv := New(cfg, log, provider.NewMock(), nil)

	// 1. Start.
	ln, err := srv.Listen()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()
	base := fmt.Sprintf("http://%s", ln.Addr().String())
	client := &http.Client{Timeout: 5 * time.Second}

	// 2. Health check.
	health, err := doRequest(t, client, http.MethodGet, base+"/health", "", "")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if health.status != http.StatusOK {
		t.Fatalf("health: got %d", health.status)
	}

	post := func(apiKey, body string) httpResult {
		res, err := doRequest(t, client, http.MethodPost, base+"/v1/chat/completions", apiKey, body)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		return res
	}

	chatBody := `{"model":"e2e-model","messages":[{"role":"user","content":"ping from e2e"}]}`

	// 3–4. No API key → 401.
	if res := post("", chatBody); res.status != http.StatusUnauthorized {
		t.Fatalf("no key: got %d, want 401", res.status)
	}
	// 5–6. Invalid key → 401.
	if res := post("not-the-key", chatBody); res.status != http.StatusUnauthorized {
		t.Fatalf("bad key: got %d, want 401", res.status)
	}

	// 7–10. Valid request → mock provider → normalized response.
	res := post(e2eKey, chatBody)
	if res.status != http.StatusOK {
		t.Fatalf("valid chat: got %d — body: %s", res.status, res.body)
	}
	var chat provider.ChatResponse
	if err := json.Unmarshal(res.body, &chat); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if chat.Object != "chat.completion" || chat.Model != "e2e-model" ||
		len(chat.Choices) != 1 || chat.Choices[0].FinishReason != "stop" {
		t.Fatalf("normalized response shape wrong: %+v", chat)
	}
	if !strings.Contains(chat.Choices[0].Message.Content, "ping from e2e") {
		t.Fatalf("mock provider did not receive/echo the request: %q", chat.Choices[0].Message.Content)
	}
	if res.header.Get("X-Request-Id") == "" {
		t.Error("response missing X-Request-Id")
	}

	// 11. Logs: request metadata present, secrets absent.
	logText := logs.String()
	for _, want := range []string{"request_id", `"method":"POST"`, "/v1/chat/completions", `"provider":"mock"`, `"model":"e2e-model"`, "duration_ms"} {
		if !strings.Contains(logText, want) {
			t.Errorf("logs missing expected metadata %q", want)
		}
	}
	if strings.Contains(logText, e2eKey) {
		t.Error("SECURITY: gateway API key appeared in logs")
	}
	if strings.Contains(logText, "ping from e2e") {
		t.Error("prompt content appeared in logs — prompts must never be logged")
	}
	if strings.Contains(logText, "Authorization") {
		t.Error("Authorization header material appeared in logs")
	}

	// 12. Graceful shutdown, with a request in flight.
	inFlight := make(chan struct{})
	var inFlightStatus int
	go func() {
		defer close(inFlight)
		res, err := doRequest(t, client, http.MethodPost, base+"/v1/chat/completions", e2eKey, chatBody)
		if err != nil {
			return
		}
		inFlightStatus = res.status
	}()
	time.Sleep(50 * time.Millisecond) // let the request enter the server

	if err := srv.Shutdown(5 * time.Second); err != nil {
		t.Fatalf("graceful shutdown failed: %v", err)
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve returned error after shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}
	<-inFlight
	if inFlightStatus != http.StatusOK {
		t.Errorf("in-flight request during shutdown: got %d, want 200", inFlightStatus)
	}

	// Post-shutdown: connections refused.
	if _, err := doRequest(t, client, http.MethodGet, base+"/health", "", ""); err == nil {
		t.Error("server still accepting connections after shutdown")
	}
}
