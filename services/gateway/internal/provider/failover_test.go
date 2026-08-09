package provider

import (
	"context"
	"testing"

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

type countingProvider struct{ calls *int }

func (c *countingProvider) Name() string { return "counting" }
func (c *countingProvider) ChatCompletion(_ context.Context, _ *ChatRequest) (*ChatResponse, error) {
	*c.calls++
	return &ChatResponse{Choices: []Choice{{Message: Message{Content: "x"}}}}, nil
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
