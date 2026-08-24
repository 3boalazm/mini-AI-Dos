package provider

import (
	"context"
	"testing"
	"time"

	"github.com/ai-dos/foundation/errors"
)

// fakeProvider returns a fixed error, or echoes its model on success.
type fakeProvider struct {
	name string
	err  error
	seen *string // records the model it was asked for, if non-nil
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) ChatCompletion(_ context.Context, req *ChatRequest) (*ChatResponse, error) {
	if f.seen != nil {
		*f.seen = req.Model
	}
	if f.err != nil {
		return nil, f.err
	}
	return &ChatResponse{
		Model:   req.Model,
		Choices: []Choice{{Message: Message{Role: RoleAssistant, Content: "from " + f.name}}},
	}, nil
}

func TestFailover_FirstSucceeds(t *testing.T) {
	var seen string
	f := NewFailover([]Upstream{
		{Name: "a", Model: "model-a", Provider: &fakeProvider{name: "a", seen: &seen}},
		{Name: "b", Model: "model-b", Provider: &fakeProvider{name: "b", err: errors.New(errors.CodeRateLimited, "x")}},
	}, testLogger())

	resp, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "caller-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Model != "model-a" {
		t.Errorf("should report the serving upstream's model, got %q", resp.Model)
	}
	if seen != "model-a" {
		t.Errorf("upstream should be called with its own model, not the caller's; got %q", seen)
	}
}

func TestFailover_FallsOverOnError(t *testing.T) {
	f := NewFailover([]Upstream{
		{Name: "a", Model: "ma", Provider: &fakeProvider{name: "a", err: errors.New(errors.CodeRateLimited, "rate")}},
		{Name: "b", Model: "mb", Provider: &fakeProvider{name: "b", err: errors.New(errors.CodeUpstream, "5xx")}},
		{Name: "c", Model: "mc", Provider: &fakeProvider{name: "c"}},
	}, testLogger())

	resp, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"})
	if err != nil {
		t.Fatalf("should have fallen over to the healthy upstream: %v", err)
	}
	if resp.Model != "mc" {
		t.Errorf("got %q, want mc (third upstream)", resp.Model)
	}
}

func TestFailover_AllFailReturnsLastError(t *testing.T) {
	f := NewFailover([]Upstream{
		{Name: "a", Model: "ma", Provider: &fakeProvider{name: "a", err: errors.New(errors.CodeRateLimited, "rate")}},
		{Name: "b", Model: "mb", Provider: &fakeProvider{name: "b", err: errors.New(errors.CodeUpstream, "down")}},
	}, testLogger())

	_, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"})
	if err == nil {
		t.Fatal("expected an error when every upstream fails")
	}
	var ae *errors.AppError
	if !asAppErr(err, &ae) || ae.Code != errors.CodeUpstream {
		t.Errorf("should surface the last upstream's error, got %v", err)
	}
}

func TestFailover_StopsOnCancelledContext(t *testing.T) {
	calls := 0
	counting := &countingProvider{calls: &calls}
	f := NewFailover([]Upstream{
		{Name: "a", Model: "ma", Provider: counting},
		{Name: "b", Model: "mb", Provider: counting},
	}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.ChatCompletion(ctx, &ChatRequest{Model: "x"}); err == nil {
		t.Fatal("cancelled context should produce an error")
	}
	if calls != 0 {
		t.Errorf("no upstream should be called once the context is cancelled, got %d", calls)
	}
}

// countingProvider counts calls, failing with err when set, otherwise
// serving and echoing the requested model.
type countingProvider struct {
	calls *int
	err   error
}

func (c *countingProvider) Name() string { return "counting" }
func (c *countingProvider) ChatCompletion(_ context.Context, req *ChatRequest) (*ChatResponse, error) {
	*c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return &ChatResponse{Model: req.Model, Choices: []Choice{{Message: Message{Content: "x"}}}}, nil
}

// asAppErr is a tiny local errors.As helper to avoid importing stderrors
// in the test just for one assertion.
func asAppErr(err error, target **errors.AppError) bool {
	if ae, ok := err.(*errors.AppError); ok {
		*target = ae
		return true
	}
	return false
}

// blockingProvider blocks until its context is cancelled, then returns
// the context's error wrapped as an upstream failure — simulates a
// hung upstream (laptop node powered off behind a live tunnel).
type blockingProvider struct{ name string }

func (b *blockingProvider) Name() string { return b.name }

func (b *blockingProvider) ChatCompletion(ctx context.Context, _ *ChatRequest) (*ChatResponse, error) {
	<-ctx.Done()
	return nil, errors.Wrap(errors.CodeTimeout, "upstream timed out", ctx.Err())
}

func TestFailover_PerUpstreamTimeout(t *testing.T) {
	f := NewFailover([]Upstream{
		{Name: "slow", Model: "ms", Provider: &blockingProvider{name: "slow"}, Timeout: 50 * time.Millisecond},
		{Name: "fast", Model: "mf", Provider: &fakeProvider{name: "fast"}},
	}, testLogger())

	start := time.Now()
	resp, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("healthy upstream should have served: %v", err)
	}
	if resp.Model != "mf" {
		t.Errorf("expected the fast upstream to serve, got %q", resp.Model)
	}
	// The slow attempt must cost ~its own timeout, not the caller's
	// patience. Generous bound for slow CI machines.
	if elapsed > 2*time.Second {
		t.Errorf("slow upstream delayed the chain %v; per-upstream timeout not applied", elapsed)
	}
}

func TestFailover_PerUpstreamTimeoutDoesNotCancelParent(t *testing.T) {
	f := NewFailover([]Upstream{
		{Name: "slow", Model: "ms", Provider: &blockingProvider{name: "slow"}, Timeout: 20 * time.Millisecond},
		{Name: "ok", Model: "mo", Provider: &fakeProvider{name: "ok"}},
	}, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := f.ChatCompletion(ctx, &ChatRequest{Model: "x"}); err != nil {
		t.Fatalf("parent context must survive an attempt timeout: %v", err)
	}
}

func TestFailover_CircuitOpensAfterThresholdAndSkips(t *testing.T) {
	calls := 0
	failing := &countingProvider{err: errors.New(errors.CodeUpstream, "down"), calls: &calls}
	f := NewFailover([]Upstream{
		{Name: "dead", Model: "md", Provider: failing},
		{Name: "ok", Model: "mo", Provider: &fakeProvider{name: "ok"}},
	}, testLogger())

	// breakerThreshold failing requests open the circuit...
	for i := 0; i < breakerThreshold; i++ {
		if _, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"}); err != nil {
			t.Fatalf("request %d should have failed over to ok: %v", i, err)
		}
	}
	if calls != breakerThreshold {
		t.Fatalf("dead upstream should have been tried %d times, got %d", breakerThreshold, calls)
	}

	// ...and while open, the dead upstream is not called at all.
	for i := 0; i < 5; i++ {
		if _, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"}); err != nil {
			t.Fatalf("request with open circuit should still serve: %v", err)
		}
	}
	if calls != breakerThreshold {
		t.Errorf("open circuit must skip the dead upstream; calls went from %d to %d", breakerThreshold, calls)
	}
}

func TestFailover_CircuitClosesAfterCooldownOnSuccess(t *testing.T) {
	calls := 0
	flaky := &countingProvider{err: errors.New(errors.CodeUpstream, "down"), calls: &calls}
	f := NewFailover([]Upstream{
		{Name: "flaky", Model: "mf", Provider: flaky},
		{Name: "ok", Model: "mo", Provider: &fakeProvider{name: "ok"}},
	}, testLogger())

	fakeNow := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	f.now = func() time.Time { return fakeNow }

	for i := 0; i < breakerThreshold; i++ {
		if _, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"}); err != nil {
			t.Fatalf("request %d should have failed over to ok: %v", i, err)
		}
	}

	// Cooldown passes and the upstream recovers.
	fakeNow = fakeNow.Add(breakerCooldown + time.Second)
	flaky.err = nil

	resp, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"})
	if err != nil {
		t.Fatalf("probe after cooldown should succeed: %v", err)
	}
	if resp.Model != "mf" {
		t.Errorf("recovered upstream should serve again, got %q", resp.Model)
	}
	if calls != breakerThreshold+1 {
		t.Errorf("expected exactly one probe call after cooldown, calls=%d", calls)
	}
}

func TestFailover_AllCircuitsOpenStillTries(t *testing.T) {
	calls := 0
	only := &countingProvider{err: errors.New(errors.CodeUpstream, "down"), calls: &calls}
	f := NewFailover([]Upstream{
		{Name: "only", Model: "mo", Provider: only},
	}, testLogger())

	for i := 0; i < breakerThreshold+2; i++ {
		if _, err := f.ChatCompletion(context.Background(), &ChatRequest{Model: "x"}); err == nil {
			t.Fatal("expected error from the only, failing upstream")
		}
	}
	// Even with its circuit open, the only upstream keeps being tried —
	// availability beats breaker purity when nothing else is healthy.
	if calls != breakerThreshold+2 {
		t.Errorf("sole upstream must always be attempted, calls=%d of %d", calls, breakerThreshold+2)
	}
}
