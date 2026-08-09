// Package agent implements the Mini AI-DOS core agent loop — phase A1
// of docs/implementation/AGENT_ROADMAP.md: Understand/Plan → Execute →
// Inspect → Fix → Result, with observable statuses and per-step
// progress. There are no tools yet; execution is model-only. Later
// phases plug tools into the execute stage without changing this
// lifecycle, which is the load-bearing contract the UI builds on.
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/provider"
)

const (
	// runTimeout bounds one whole run end to end — a run is several
	// model calls, each already bounded by the provider client timeout.
	runTimeout = 15 * time.Minute
	// maxSteps caps a parsed plan; more than this is model rambling,
	// not planning.
	maxSteps = 6
)

// Status is the run lifecycle. The progression is fixed:
// planning → executing → inspecting → (fixing) → completed | failed.
type Status string

const (
	StatusPlanning   Status = "planning"
	StatusExecuting  Status = "executing"
	StatusInspecting Status = "inspecting"
	StatusFixing     Status = "fixing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// Step states as shown to the user.
const (
	StepPending = "pending"
	StepActive  = "active"
	StepDone    = "done"
)

// Step is one planned unit of work.
type Step struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// Run is one agent task from request to result. Snapshots returned by
// the engine are copies — callers never share memory with the loop.
type Run struct {
	ID      string `json:"id"`
	Task    string `json:"task"`
	Status  Status `json:"status"`
	Steps   []Step `json:"steps"`
	Result  string `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
	Created int64  `json:"created"`

	cancel context.CancelFunc
}

// Engine owns every run and drives the loop. One engine per server.
type Engine struct {
	provider provider.Provider
	model    string
	log      *logging.Logger

	mu   sync.RWMutex
	runs map[string]*Run
}

// NewEngine builds the engine. model is the upstream model every agent
// call uses (the AI_MODEL default); Start rejects tasks when empty,
// because unlike /v1/chat/completions there is no per-request model.
func NewEngine(p provider.Provider, model string, log *logging.Logger) *Engine {
	return &Engine{provider: p, model: model, log: log, runs: make(map[string]*Run)}
}

// Start validates and launches a run, returning its first snapshot.
func (e *Engine) Start(task string) (*Run, error) {
	if strings.TrimSpace(task) == "" {
		return nil, fmt.Errorf("task must be a non-empty string")
	}
	if e.model == "" {
		return nil, fmt.Errorf("agent runs need AI_MODEL configured on the gateway")
	}

	id := "run_" + newID()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	run := &Run{
		ID:      id,
		Task:    task,
		Status:  StatusPlanning,
		Created: time.Now().Unix(),
		cancel:  cancel,
	}
	e.mu.Lock()
	e.runs[id] = run
	e.mu.Unlock()

	go e.loop(ctx, id, task)
	return e.Get(id), nil
}

// Get returns a snapshot copy of a run, or nil when unknown.
func (e *Engine) Get(id string) *Run {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.runs[id]
	if !ok {
		return nil
	}
	cp := *r
	cp.cancel = nil
	cp.Steps = append([]Step(nil), r.Steps...)
	return &cp
}

// Cancel aborts a running run. Reports whether the id was known.
func (e *Engine) Cancel(id string) bool {
	e.mu.RLock()
	r, ok := e.runs[id]
	e.mu.RUnlock()
	if !ok {
		return false
	}
	r.cancel()
	return true
}

// update applies fn to a run under the write lock.
func (e *Engine) update(id string, fn func(*Run)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.runs[id]; ok {
		fn(r)
	}
}

// loop is the whole phase-A1 agent: plan, execute step by step,
// inspect the combined result, fix once if inspection objects.
func (e *Engine) loop(ctx context.Context, id, task string) {
	log := e.log
	fail := func(err error) {
		msg := err.Error()
		if ctx.Err() != nil {
			msg = "الرن اتلغى"
		}
		e.update(id, func(r *Run) { r.Status = StatusFailed; r.Error = msg })
		log.Warn("agent run ended without completing", "run_id", id, "error", msg)
	}

	titles, err := e.plan(ctx, task)
	if err != nil {
		fail(err)
		return
	}
	steps := make([]Step, len(titles))
	for i, t := range titles {
		steps[i] = Step{Title: t, Status: StepPending}
	}
	e.update(id, func(r *Run) { r.Steps = steps; r.Status = StatusExecuting })

	var work strings.Builder
	for i, title := range titles {
		e.update(id, func(r *Run) { r.Steps[i].Status = StepActive })
		out, err := e.executeStep(ctx, task, titles, i, work.String())
		if err != nil {
			fail(err)
			return
		}
		if work.Len() > 0 {
			work.WriteString("\n\n")
		}
		work.WriteString(out)
		e.update(id, func(r *Run) { r.Steps[i].Status = StepDone })
		log.Info("agent step done", "run_id", id, "step", i+1, "title", title)
	}

	e.update(id, func(r *Run) { r.Status = StatusInspecting })
	verdict, err := e.inspect(ctx, task, work.String())
	if err != nil {
		fail(err)
		return
	}

	result := work.String()
	if !inspectionPassed(verdict) {
		e.update(id, func(r *Run) { r.Status = StatusFixing })
		result, err = e.fix(ctx, task, work.String(), verdict)
		if err != nil {
			fail(err)
			return
		}
	}

	e.update(id, func(r *Run) { r.Status = StatusCompleted; r.Result = result })
	log.Info("agent run completed", "run_id", id, "steps", len(titles), "fixed", !inspectionPassed(verdict))
}

// chat is one bounded model call through the provider seam.
func (e *Engine) chat(ctx context.Context, system, user string) (string, error) {
	resp, err := e.provider.ChatCompletion(ctx, &provider.ChatRequest{
		Model: e.model,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: system},
			{Role: provider.RoleUser, Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("upstream returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

const planSystem = "You are the planning module of a build agent. Reply with ONLY a JSON array of 3 to 6 short step titles (plain strings, in the same language as the task) that break the task into concrete build steps. No prose, no code fences."

func (e *Engine) plan(ctx context.Context, task string) ([]string, error) {
	raw, err := e.chat(ctx, planSystem, task)
	if err != nil {
		return nil, err
	}
	if titles := parsePlan(raw); len(titles) > 0 {
		return titles, nil
	}
	// A model that can't produce a parseable plan still gets one
	// honest step — the loop must not depend on model formatting.
	return []string{"تنفيذ المهمة"}, nil
}

const execSystem = "You are the execution module of a build agent working through a plan step by step. Produce the deliverable content for the CURRENT step only, building on the work so far without repeating it. Output content only — no meta commentary."

func (e *Engine) executeStep(ctx context.Context, task string, titles []string, i int, workSoFar string) (string, error) {
	var b strings.Builder
	b.WriteString("TASK:\n" + task + "\n\nPLAN:\n")
	for n, t := range titles {
		fmt.Fprintf(&b, "%d. %s\n", n+1, t)
	}
	if workSoFar != "" {
		b.WriteString("\nWORK SO FAR:\n" + workSoFar + "\n")
	}
	fmt.Fprintf(&b, "\nCURRENT STEP (%d): %s", i+1, titles[i])
	return e.chat(ctx, execSystem, b.String())
}

const inspectSystem = "You are the inspection module of a build agent. Compare the deliverable to the task. If it fulfils the task, reply with exactly OK. Otherwise list the concrete problems, briefly."

func (e *Engine) inspect(ctx context.Context, task, deliverable string) (string, error) {
	return e.chat(ctx, inspectSystem, "TASK:\n"+task+"\n\nDELIVERABLE:\n"+deliverable)
}

const fixSystem = "You are the fixing module of a build agent. Given the task, the deliverable, and the inspection notes, output the corrected COMPLETE final deliverable only — no commentary."

func (e *Engine) fix(ctx context.Context, task, deliverable, notes string) (string, error) {
	return e.chat(ctx, fixSystem, "TASK:\n"+task+"\n\nDELIVERABLE:\n"+deliverable+"\n\nINSPECTION NOTES:\n"+notes)
}

// inspectionPassed treats only a clear leading OK as a pass.
func inspectionPassed(verdict string) bool {
	v := strings.TrimSpace(verdict)
	return v == "OK" || strings.HasPrefix(v, "OK\n") || strings.HasPrefix(v, "OK ")
}

// parsePlan extracts a JSON string array from raw model output,
// tolerating prose or fences around it. Returns nil when nothing
// usable is found.
func parsePlan(raw string) []string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil
	}
	var titles []string
	if err := json.Unmarshal([]byte(raw[start:end+1]), &titles); err != nil {
		return nil
	}
	out := titles[:0]
	for _, t := range titles {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	if len(out) > maxSteps {
		out = out[:maxSteps]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Timestamp fallback keeps ids unique enough for an in-memory
		// store; crypto/rand failing is already a broken host.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
