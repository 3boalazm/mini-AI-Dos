package provider

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/foundation/logging"
)

const (
	// maxResponseBytes bounds how much of an upstream response body the
	// gateway will read — a misbehaving upstream cannot exhaust memory.
	maxResponseBytes = 10 << 20 // 10 MiB
	// maxUpstreamErrorLen bounds how much upstream error text is passed
	// through to callers — enough to be useful, never a page dump.
	maxUpstreamErrorLen = 300
)

// OpenAI is an HTTP provider speaking the OpenAI Chat Completions wire
// protocol. Any OpenAI-compatible endpoint works via BaseURL.
type OpenAI struct {
	baseURL string
	apiKey  string
	client  *http.Client
	log     *logging.Logger
}

// NewOpenAI builds the provider. The API key is held privately and is
// never logged or serialized by anything in this package.
func NewOpenAI(baseURL, apiKey string, timeout time.Duration, log *logging.Logger) *OpenAI {
	return &OpenAI{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
		log:     log,
	}
}

func (o *OpenAI) Name() string { return "openai" }

// upstreamError is the OpenAI error body shape, decoded only to pass a
// short human-readable reason through to the caller.
type upstreamError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (o *OpenAI) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "failed to encode upstream request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "failed to build upstream request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		// Timeout (client-level or context deadline) versus any other
		// transport failure — the two say different things to a caller.
		if ctx.Err() != nil || isTimeout(err) {
			o.log.FromContext(ctx).Warn("upstream request timed out", "provider", o.Name())
			return nil, errors.Wrap(errors.CodeTimeout, "upstream provider timed out", err)
		}
		o.log.FromContext(ctx).Warn("upstream request failed", "provider", o.Name())
		return nil, errors.Wrap(errors.CodeUpstream, "could not reach upstream provider", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, errors.Wrap(errors.CodeUpstream, "failed reading upstream response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, o.mapUpstreamStatus(ctx, resp.StatusCode, raw)
	}

	var out ChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		o.log.FromContext(ctx).Warn("upstream returned malformed JSON", "provider", o.Name(), "status", resp.StatusCode)
		return nil, errors.Wrap(errors.CodeUpstream, "upstream provider returned malformed response", err)
	}
	if len(out.Choices) == 0 {
		return nil, errors.New(errors.CodeUpstream, "upstream provider returned no choices")
	}
	if out.Object == "" {
		out.Object = "chat.completion"
	}
	return &out, nil
}

// mapUpstreamStatus turns a non-200 upstream status into the AppError
// the HTTP layer serializes. The mapping is deliberate:
//   - 400/404: the caller's request was genuinely invalid (bad params,
//     unknown model) — pass the status through so the caller can fix it.
//   - 401/403: OUR upstream credential is wrong — that is a gateway
//     misconfiguration, so callers see 502, never 401 (their own key
//     was already accepted).
//   - 429: provider throttling, passed through as 429.
//   - anything else: 502.
func (o *OpenAI) mapUpstreamStatus(ctx context.Context, status int, raw []byte) error {
	reason := upstreamReason(raw)
	log := o.log.FromContext(ctx)

	switch {
	case status == http.StatusBadRequest:
		return errors.New(errors.CodeValidation, "upstream rejected request: "+reason)
	case status == http.StatusNotFound:
		return errors.New(errors.CodeNotFound, "upstream resource not found: "+reason)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		// Never echo upstream auth details to the caller.
		log.Error("upstream rejected gateway credentials — check AI_API_KEY", "provider", o.Name(), "status", status)
		return errors.New(errors.CodeUpstream, "upstream provider rejected the gateway's credentials")
	case status == http.StatusTooManyRequests:
		log.Warn("upstream rate limit hit", "provider", o.Name())
		return errors.New(errors.CodeRateLimited, "upstream provider rate limit exceeded")
	default:
		log.Warn("upstream returned error status", "provider", o.Name(), "status", status)
		return errors.New(errors.CodeUpstream, fmt.Sprintf("upstream provider returned status %d: %s", status, reason))
	}
}

// upstreamReason extracts a short reason string from an upstream error
// body, tolerating any shape. Truncated, never a raw dump.
func upstreamReason(raw []byte) string {
	var ue upstreamError
	if err := json.Unmarshal(raw, &ue); err == nil && ue.Error.Message != "" {
		return truncate(ue.Error.Message, maxUpstreamErrorLen)
	}
	return "no detail provided"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func isTimeout(err error) bool {
	var ne net.Error
	if stderrors.As(err, &ne) {
		return ne.Timeout()
	}
	return false
}
