package provider

import (
	"context"
	"sync"
	"time"

	"github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/foundation/logging"
)

// Circuit breaker tuning. Request-driven half-open: once cooldown
// elapses, the next request that reaches this upstream IS the probe —
// success closes the circuit, failure re-opens it immediately (the
// failure count is already at the threshold). No background goroutine.
const (
	// breakerThreshold consecutive failures open an upstream's circuit.
	breakerThreshold = 3
	// breakerCooldown is how long an open circuit is skipped before the
	// next request is allowed through as a probe.
	breakerCooldown = 30 * time.Second
)

// Upstream is one backend in a failover chain: a provider plus the
// model ID to use with it. Model IDs differ per provider (Gemini's
// "gemini-3.6-flash" vs Groq's "openai/gpt-oss-120b"), so the chain
// carries each upstream's own model rather than trusting the caller's.
// Timeout bounds one attempt against this upstream specifically; zero
// means no per-upstream bound beyond the caller's context. A slow
// local upstream (a laptop node behind a tunnel) gets a tight timeout
// so its failure costs the chain seconds, not the global AI_TIMEOUT.
type Upstream struct {
	Name     string
	Model    string
	Provider Provider
	Timeout  time.Duration
}

// breakerState tracks one upstream's consecutive failures and, when
// the circuit is open, until when it should be skipped.
type breakerState struct {
	fails     int
	openUntil time.Time
}

// Failover tries a list of upstreams in order, moving to the next when
// one errors — the whole point of the free-first stack: a single
// provider's per-minute rate limit no longer fails a request when four
// others are ready. It is itself a Provider, so nothing above it knows
// there is more than one backend.
//
// Each upstream has a circuit breaker: breakerThreshold consecutive
// failures and it is skipped for breakerCooldown, so a dead upstream
// (laptop node powered off) stops adding its timeout to every request.
// If every circuit is open the chain tries all upstreams anyway —
// availability wins over breaker purity when nothing is healthy.
type Failover struct {
	upstreams []Upstream
	log       *logging.Logger
	// now is a seam for tests; time.Now in production.
	now func() time.Time

	mu    sync.Mutex
	state []breakerState
}

// NewFailover builds a failover provider over the given ordered chain.
func NewFailover(upstreams []Upstream, log *logging.Logger) *Failover {
	return &Failover{
		upstreams: upstreams,
		log:       log,
		now:       time.Now,
		state:     make([]breakerState, len(upstreams)),
	}
}

func (f *Failover) Name() string { return "failover" }

// available returns the indexes to try: upstreams with closed (or
// cooled-down) circuits, or every upstream when all circuits are open.
func (f *Failover) available() []int {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := f.now()
	closed := make([]int, 0, len(f.upstreams))
	for i := range f.upstreams {
		if now.After(f.state[i].openUntil) {
			closed = append(closed, i)
		}
	}
	if len(closed) > 0 {
		return closed
	}
	all := make([]int, len(f.upstreams))
	for i := range all {
		all[i] = i
	}
	return all
}

// recordSuccess closes the upstream's circuit.
func (f *Failover) recordSuccess(i int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state[i] = breakerState{}
}

// recordFailure counts a failure and opens the circuit at the
// threshold. Returns true when this call opened (or re-opened) it.
func (f *Failover) recordFailure(i int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state[i].fails++
	if f.state[i].fails >= breakerThreshold {
		f.state[i].openUntil = f.now().Add(breakerCooldown)
		return true
	}
	return false
}

// ChatCompletion tries each available upstream in order. Each attempt
// uses that upstream's own model (overriding the caller's, since model
// IDs are provider-specific) and its own timeout, and the returned
// response reports the model that actually served — honest telemetry
// the UI surfaces. Any error falls over to the next upstream; a
// cancelled caller context stops immediately rather than hammering the
// rest of the chain.
func (f *Failover) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	var lastErr error
	for _, i := range f.available() {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(errors.CodeTimeout, "request cancelled during failover", err)
		}
		up := f.upstreams[i]

		attempt := *req
		attempt.Model = up.Model

		attemptCtx := ctx
		cancel := context.CancelFunc(func() {})
		if up.Timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, up.Timeout)
		}
		resp, err := up.Provider.ChatCompletion(attemptCtx, &attempt)
		cancel()

		if err == nil {
			f.recordSuccess(i)
			return resp, nil
		}
		lastErr = err
		opened := f.recordFailure(i)
		f.log.FromContext(ctx).Warn("upstream failed, falling over",
			"provider", up.Name, "model", up.Model, "error", err.Error())
		if opened {
			f.log.FromContext(ctx).Warn("circuit opened for upstream",
				"provider", up.Name, "cooldown", breakerCooldown.String(),
				"consecutive_failures", breakerThreshold)
		}
	}
	if lastErr == nil {
		return nil, errors.New(errors.CodeUpstream, "no upstream providers configured")
	}
	return nil, lastErr
}
