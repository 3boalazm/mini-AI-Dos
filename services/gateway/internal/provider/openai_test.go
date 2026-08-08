package provider

import (
	"context"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/foundation/logging"
)

func stderrorsAs(err error, target any) bool { return stderrors.As(err, target) }

func testLogger() *logging.Logger {
	return logging.New(logging.Config{Environment: logging.EnvDevelopment, Output: io.Discard})
}

func chatReq() *ChatRequest {
	return &ChatRequest{Model: "gpt-test", Messages: []Message{{Role: RoleUser, Content: "hello"}}}
}

// upstream builds an httptest server and an OpenAI provider pointed at it.
func upstream(t *testing.T, handler http.HandlerFunc) *OpenAI {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewOpenAI(srv.URL, "sk-test-upstream", 2*time.Second, testLogger())
}

func appCode(t *testing.T, err error) errors.Code {
	t.Helper()
	var appErr *errors.AppError
	if !stderrorsAs(err, &appErr) {
		t.Fatalf("expected *errors.AppError, got %T: %v", err, err)
	}
	return appErr.Code
}

func TestOpenAI_Success(t *testing.T) {
	var gotAuth, gotPath string
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-abc", "object": "chat.completion", "created": 123, "model": "gpt-test",
			"choices": [{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}],
			"usage": {"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
		}`))
	})

	resp, err := p.ChatCompletion(context.Background(), chatReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer sk-test-upstream" {
		t.Errorf("upstream auth header wrong: %q", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path: got %q", gotPath)
	}
	if resp.ID != "chatcmpl-abc" || resp.Choices[0].Message.Content != "hi there" {
		t.Errorf("response not passed through: %+v", resp)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 3 {
		t.Errorf("usage should pass through: %+v", resp.Usage)
	}
}

func TestOpenAI_UsageAbsentStaysAbsent(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"x","created":1,"model":"m",
			"choices":[{"index":0,"message":{"role":"assistant","content":"y"},"finish_reason":"stop"}]}`))
	})
	resp, err := p.ChatCompletion(context.Background(), chatReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Usage != nil {
		t.Errorf("usage must not be invented when upstream omits it, got %+v", resp.Usage)
	}
}

func TestOpenAI_Upstream401And403_BecomeUpstreamErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"bad key sk-secret-value"}}`))
		})
		_, err := p.ChatCompletion(context.Background(), chatReq())
		if code := appCode(t, err); code != errors.CodeUpstream {
			t.Errorf("status %d: got code %s, want upstream_error (gateway misconfig, not caller's 401)", status, code)
		}
		// The upstream's own auth error text must never reach a caller.
		if strings.Contains(err.Error(), "sk-secret-value") {
			t.Errorf("status %d: upstream auth detail leaked into error: %v", status, err)
		}
	}
}

func TestOpenAI_Upstream429_PassesThrough(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := p.ChatCompletion(context.Background(), chatReq())
	if code := appCode(t, err); code != errors.CodeRateLimited {
		t.Errorf("got code %s, want rate_limited", code)
	}
}

func TestOpenAI_Upstream500_BecomesUpstreamError(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream exploded"}}`))
	})
	_, err := p.ChatCompletion(context.Background(), chatReq())
	if code := appCode(t, err); code != errors.CodeUpstream {
		t.Errorf("got code %s, want upstream_error", code)
	}
	if !strings.Contains(err.Error(), "upstream exploded") {
		t.Errorf("short upstream reason should be preserved for diagnosis: %v", err)
	}
}

func TestOpenAI_Upstream400_BecomesValidation(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"max_tokens too large"}}`))
	})
	_, err := p.ChatCompletion(context.Background(), chatReq())
	if code := appCode(t, err); code != errors.CodeValidation {
		t.Errorf("got code %s, want validation_error", code)
	}
}

func TestOpenAI_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	t.Cleanup(srv.Close)
	p := NewOpenAI(srv.URL, "k", 50*time.Millisecond, testLogger())

	_, err := p.ChatCompletion(context.Background(), chatReq())
	if code := appCode(t, err); code != errors.CodeTimeout {
		t.Errorf("got code %s, want timeout", code)
	}
}

func TestOpenAI_ContextCancellation(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	p := NewOpenAI(srv.URL, "k", 10*time.Second, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.ChatCompletion(ctx, chatReq())
	if code := appCode(t, err); code != errors.CodeTimeout {
		t.Errorf("got code %s, want timeout for context deadline", code)
	}
}

func TestOpenAI_MalformedJSON(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id": "half a response`))
	})
	_, err := p.ChatCompletion(context.Background(), chatReq())
	if code := appCode(t, err); code != errors.CodeUpstream {
		t.Errorf("got code %s, want upstream_error for malformed JSON", code)
	}
}

func TestOpenAI_EmptyChoicesRejected(t *testing.T) {
	p := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"x","created":1,"model":"m","choices":[]}`))
	})
	_, err := p.ChatCompletion(context.Background(), chatReq())
	if code := appCode(t, err); code != errors.CodeUpstream {
		t.Errorf("got code %s, want upstream_error for empty choices", code)
	}
}

func TestOpenAI_NetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // dead endpoint: connection refused
	p := NewOpenAI(srv.URL, "k", time.Second, testLogger())

	_, err := p.ChatCompletion(context.Background(), chatReq())
	if code := appCode(t, err); code != errors.CodeUpstream {
		t.Errorf("got code %s, want upstream_error for network failure", code)
	}
}
