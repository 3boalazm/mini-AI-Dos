package provider

import (
	"context"

	"github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/foundation/logging"
)

// Upstream is one backend in a failover chain: a provider plus the
// model ID to use with it. Model IDs differ per provider (Gemini's
// "gemini-3.6-flash" vs Groq's "openai/gpt-oss-120b"), so the chain
// carries each upstream's own model rather than trusting the caller's.
type Upstream struct {
	Name     string
	Model    string
	Provider Provider
}

// Failover tries a list of upstreams in order, moving to the next when
// one errors — the whole point of the free-first stack: a single
// provider's per-minute rate limit no longer fails a request when four
// others are ready. It is itself a Provider, so nothing above it knows
// there is more than one backend.
type Failover struct {
	upstreams []Upstream
	log       *logging.Logger
}

// NewFailover builds a failover provider over the given ordered chain.
func NewFailover(upstreams []Upstream, log *logging.Logger) *Failover {
	return &Failover{upstreams: upstreams, log: log}
}

func (f *Failover) Name() string { return "failover" }

// ChatCompletion tries each upstream in order. Each attempt uses that
// upstream's own model (overriding the caller's, since model IDs are
// provider-specific), and the returned response reports the model that
// actually served — honest telemetry the UI surfaces. Any error falls
// over to the next upstream; a cancelled context stops immediately
// rather than hammering the rest of the chain.
func (f *Failover) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	var lastErr error
	for _, up := range f.upstreams {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(errors.CodeTimeout, "request cancelled during failover", err)
		}
		attempt := *req
		attempt.Model = up.Model
		resp, err := up.Provider.ChatCompletion(ctx, &attempt)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		f.log.FromContext(ctx).Warn("upstream failed, falling over",
			"provider", up.Name, "model", up.Model, "error", err.Error())
	}
	if lastErr == nil {
		return nil, errors.New(errors.CodeUpstream, "no upstream providers configured")
	}
	return nil, lastErr
}
