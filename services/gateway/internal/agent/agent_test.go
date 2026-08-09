package agent

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	apperrors "github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/provider"
)

// scriptedProvider returns canned responses in order, cycling on the
// last one — enough to steer the loop through specific paths.
type scriptedProvider struct {
	responses []string
	calls     int
}

func (s *scriptedProvider) Name() string { return "scripted" }

func (s *scriptedProvider) ChatCompletion(_ context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	i := s.calls
	if i >= len(s.responses) {
		i = len(s.responses) - 1
	}
	s.calls++
	return &provider.ChatResponse{
		Model:   req.Model,
		Choices: []provider.Choice{{Message: provider.Message{Role: provider.RoleAssistant, Content: s.responses[i]}}},
	}, nil
}

func testLog() *logging.Logger {
	return logging.New(logging.Config{Environment: logging.EnvDevelopment, Output: io.Discard})
}

func waitFor(t *testing.T, e *Engine, id string, want Status) *Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r := e.Get(id); r != nil && (r.Status == want || r.Status == StatusFailed) {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never reached %s", id, want)
	return nil
}

func TestParsePlan(t *testing.T) {
	cases := map[string][]string{
		`["a","b","c"]`: {"a", "b", "c"},
		"Sure! Here is the plan:\n[\"x\", \"y\"]":   {"x", "y"},
		`["one","","  ","two"]`:                     {"one", "two"},
		`["1","2","3","4","5","6","7","8"]`:         {"1", "2", "3", "4", "5"},
		"no json here":                              nil,
		`{"steps":["not","an","array","of","top"]}`: {"not", "an", "array", "of", "top"},
	}
	for raw, want := range cases {
		got := parsePlan(raw)
		if len(got) != len(want) {
			t.Errorf("parsePlan(%q): got %v, want %v", raw, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parsePlan(%q)[%d]: got %q, want %q", raw, i, got[i], want[i])
			}
		}
	}
}

func TestRun_HappyPath_PlanExecuteInspectOK(t *testing.T) {
	p := &scriptedProvider{responses: []string{
		`["step one","step two"]`, // plan
		"work for step one",       // execute 1
		"work for step two",       // execute 2
		"OK",                      // inspect passes → no fix call
	}}
	e := NewEngine(p, "test-model", t.TempDir(), testLog())

	run, err := e.Start("build something")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	final := waitFor(t, e, run.ID, StatusCompleted)
	if final.Status != StatusCompleted {
		t.Fatalf("status: got %s (error %q), want completed", final.Status, final.Error)
	}
	if len(final.Steps) != 2 || final.Steps[0].Status != StepDone || final.Steps[1].Status != StepDone {
		t.Errorf("steps not all done: %+v", final.Steps)
	}
	if !strings.Contains(final.Result, "work for step one") || !strings.Contains(final.Result, "work for step two") {
		t.Errorf("result should accumulate step outputs, got %q", final.Result)
	}
	if p.calls != 4 {
		t.Errorf("call count: got %d, want 4 (no fix call when inspection passes)", p.calls)
	}
}

func TestRun_InspectionFailure_TriggersFix(t *testing.T) {
	p := &scriptedProvider{responses: []string{
		"not a plan at all",  // plan → fallback single step
		"first draft",        // execute the fallback step
		"the navbar is off",  // inspect objects
		"fixed final result", // fix output becomes the result
	}}
	e := NewEngine(p, "test-model", t.TempDir(), testLog())

	run, err := e.Start("build a page")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	final := waitFor(t, e, run.ID, StatusCompleted)
	if final.Status != StatusCompleted {
		t.Fatalf("status: got %s (error %q), want completed", final.Status, final.Error)
	}
	if len(final.Steps) != 1 {
		t.Errorf("fallback plan should be one step, got %+v", final.Steps)
	}
	if final.Result != "fixed final result" {
		t.Errorf("result should be the fixed deliverable, got %q", final.Result)
	}
}

func TestRun_ToolLoop_ProducesFiles(t *testing.T) {
	p := &scriptedProvider{responses: []string{
		`["create the page"]`, // plan → one step
		`{"tool":"write_file","args":{"path":"index.html","content":"<h1>hi</h1>"}}`, // execute: write
		`{"tool":"done","args":{"summary":"wrote index.html"}}`,                      // execute: done
		"OK", // inspect passes
	}}
	e := NewEngine(p, "test-model", t.TempDir(), testLog())

	run, err := e.Start("build a page")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	final := waitFor(t, e, run.ID, StatusCompleted)
	if final.Status != StatusCompleted {
		t.Fatalf("status: got %s (error %q), want completed", final.Status, final.Error)
	}
	if len(final.Files) != 1 || final.Files[0] != "index.html" {
		t.Fatalf("workspace files: got %v, want [index.html]", final.Files)
	}
	if !strings.Contains(final.Result, "index.html") {
		t.Errorf("result should mention the built file, got %q", final.Result)
	}

	// The file must be readable back through the engine.
	content, known, err := e.ReadRunFile(run.ID, "index.html")
	if !known || err != nil {
		t.Fatalf("ReadRunFile: known=%v err=%v", known, err)
	}
	if content != "<h1>hi</h1>" {
		t.Errorf("file content: got %q", content)
	}
}

// flakyProvider fails with a rate-limit error the first failN calls,
// then serves from responses.
type flakyProvider struct {
	mu        sync.Mutex
	failN     int
	calls     int
	responses []string
	served    int
}

func (f *flakyProvider) Name() string { return "flaky" }

func (f *flakyProvider) ChatCompletion(_ context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failN {
		return nil, apperrors.New(apperrors.CodeRateLimited, "upstream provider rate limit exceeded")
	}
	i := f.served
	if i >= len(f.responses) {
		i = len(f.responses) - 1
	}
	f.served++
	return &provider.ChatResponse{
		Model:   req.Model,
		Choices: []provider.Choice{{Message: provider.Message{Role: provider.RoleAssistant, Content: f.responses[i]}}},
	}, nil
}

func TestChatMessages_RetriesRateLimit(t *testing.T) {
	p := &flakyProvider{failN: 2, responses: []string{"recovered"}}
	e := NewEngine(p, "m", t.TempDir(), testLog())
	e.backoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond} // fast

	out, err := e.chatMessages(context.Background(), []provider.Message{{Role: provider.RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("should have recovered after retries: %v", err)
	}
	if out != "recovered" {
		t.Errorf("got %q, want recovered", out)
	}
	if p.calls != 3 {
		t.Errorf("expected 2 failures + 1 success = 3 calls, got %d", p.calls)
	}
}

func TestChatMessages_RateLimitExhausted(t *testing.T) {
	p := &flakyProvider{failN: 100, responses: []string{"never"}}
	e := NewEngine(p, "m", t.TempDir(), testLog())
	e.backoff = []time.Duration{time.Millisecond} // one retry, then give up

	if _, err := e.chatMessages(context.Background(), []provider.Message{{Role: provider.RoleUser, Content: "hi"}}); err == nil {
		t.Fatal("should return the rate-limit error after exhausting retries")
	}
	if p.calls != 2 {
		t.Errorf("expected initial + 1 retry = 2 calls, got %d", p.calls)
	}
}

func TestRun_ActivityLogRecordsTools(t *testing.T) {
	p := &scriptedProvider{responses: []string{
		`["build"]`,
		`{"tool":"write_file","args":{"path":"a.txt","content":"x"}}`,
		`{"tool":"run_command","args":{"command":"echo hi"}}`,
		`{"tool":"done","args":{"summary":"done"}}`,
		"OK",
	}}
	e := NewEngine(p, "m", t.TempDir(), testLog())
	run, _ := e.Start("t")
	final := waitFor(t, e, run.ID, StatusCompleted)
	joined := strings.Join(final.Log, "\n")
	if !strings.Contains(joined, "write_file: a.txt") || !strings.Contains(joined, "$ echo hi") {
		t.Errorf("activity log should record tool actions, got %v", final.Log)
	}
}

func TestStart_Validation(t *testing.T) {
	e := NewEngine(&scriptedProvider{responses: []string{"x"}}, "test-model", t.TempDir(), testLog())
	if _, err := e.Start("   "); err == nil {
		t.Error("empty task should be rejected")
	}
	noModel := NewEngine(&scriptedProvider{responses: []string{"x"}}, "", t.TempDir(), testLog())
	if _, err := noModel.Start("task"); err == nil {
		t.Error("engine without a model should reject runs")
	}
}

func TestGet_UnknownAndSnapshotIsolation(t *testing.T) {
	e := NewEngine(&scriptedProvider{responses: []string{`["a"]`, "w", "OK"}}, "m", t.TempDir(), testLog())
	if e.Get("nope") != nil {
		t.Error("unknown id should return nil")
	}
	run, _ := e.Start("t")
	final := waitFor(t, e, run.ID, StatusCompleted)
	final.Steps[0].Status = "tampered"
	if e.Get(run.ID).Steps[0].Status == "tampered" {
		t.Error("Get must return copies, not shared memory")
	}
}
