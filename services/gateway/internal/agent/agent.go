// Package agent implements the Mini AI-DOS core agent loop —
// Understand/Plan → Execute → Inspect → Fix → Result, with observable
// statuses and per-step progress (docs/implementation/AGENT_ROADMAP.md).
//
// Phase A1 was model-only. Phase A2 gives the loop hands: each run owns
// an isolated Workspace, and the execute and fix stages drive a text
// JSON tool protocol (read/write/edit/list/search — see tools.go) so
// the agent produces real files, not a code block in a chat message.
// The lifecycle and its statuses are unchanged; tools plug into the
// execute stage, which is exactly the contract the UI was built on.
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/provider"
)

const (
	// runTimeout bounds one whole run end to end — a run is many model
	// calls, each already bounded by the provider client timeout.
	runTimeout = 15 * time.Minute
	// maxSteps caps a parsed plan; more than this is model rambling.
	maxSteps = 5
	// maxToolCalls bounds one execute/fix stage's tool loop, so a run's
	// upstream cost stays predictable.
	maxToolCalls = 6
	// maxToolResult bounds tool output fed back to the model, so a
	// large read_file cannot blow the next turn's context.
	maxToolResult = 4000
	// inspectDumpLimit bounds the workspace snapshot shown to the
	// inspection stage.
	inspectDumpLimit = 16 * 1024
	// maxRuns caps retained runs (and their on-disk workspaces); the
	// oldest is evicted past this.
	maxRuns = 25
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
	ID      string   `json:"id"`
	Task    string   `json:"task"`
	Status  Status   `json:"status"`
	Steps   []Step   `json:"steps"`
	Files   []string `json:"files"`
	Result  string   `json:"result,omitempty"`
	Error   string   `json:"error,omitempty"`
	Created int64    `json:"created"`

	cancel context.CancelFunc
	ws     *Workspace
}

// Engine owns every run and drives the loop. One engine per server.
type Engine struct {
	provider provider.Provider
	model    string
	baseDir  string
	log      *logging.Logger

	mu   sync.RWMutex
	runs map[string]*Run
}

// NewEngine builds the engine. model is the upstream model every agent
// call uses (the AI_MODEL default). baseDir roots per-run workspaces;
// empty falls back to a temp directory.
func NewEngine(p provider.Provider, model, baseDir string, log *logging.Logger) *Engine {
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), "aidos-workspaces")
	}
	return &Engine{provider: p, model: model, baseDir: baseDir, log: log, runs: make(map[string]*Run)}
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
	ws, err := newWorkspace(e.baseDir, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	run := &Run{
		ID:      id,
		Task:    task,
		Status:  StatusPlanning,
		Created: time.Now().Unix(),
		cancel:  cancel,
		ws:      ws,
	}
	e.mu.Lock()
	e.runs[id] = run
	e.evictLocked()
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
	cp.ws = nil
	cp.Steps = append([]Step(nil), r.Steps...)
	cp.Files = append([]string(nil), r.Files...)
	return &cp
}

// ReadRunFile returns one file from a run's workspace. known reports
// whether the run id exists, so a missing run and a missing file are
// distinguishable to the HTTP layer.
func (e *Engine) ReadRunFile(id, path string) (content string, known bool, err error) {
	e.mu.RLock()
	r, ok := e.runs[id]
	e.mu.RUnlock()
	if !ok || r.ws == nil {
		return "", false, nil
	}
	c, readErr := r.ws.ReadFile(path)
	return c, true, readErr
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

// evictLocked removes the oldest run (and its workspace) once the cap
// is exceeded. Caller holds the write lock.
func (e *Engine) evictLocked() {
	if len(e.runs) <= maxRuns {
		return
	}
	var oldestID string
	var oldest int64
	first := true
	for id, r := range e.runs {
		if first || r.Created < oldest {
			oldest, oldestID, first = r.Created, id, false
		}
	}
	if r, ok := e.runs[oldestID]; ok {
		if r.ws != nil {
			_ = r.ws.Remove()
		}
		delete(e.runs, oldestID)
	}
}

// update applies fn to a run under the write lock.
func (e *Engine) update(id string, fn func(*Run)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.runs[id]; ok {
		fn(r)
	}
}

// wsOf returns a run's workspace handle (set once at Start).
func (e *Engine) wsOf(id string) *Workspace {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if r, ok := e.runs[id]; ok {
		return r.ws
	}
	return nil
}

// loop is the whole agent: plan, execute each step with tools,
// inspect the workspace, fix once if inspection objects.
func (e *Engine) loop(ctx context.Context, id, task string) {
	fail := func(err error) {
		msg := err.Error()
		if ctx.Err() != nil {
			msg = "الرن اتلغى"
		}
		e.update(id, func(r *Run) { r.Status = StatusFailed; r.Error = msg })
		e.log.Warn("agent run ended without completing", "run_id", id, "error", msg)
	}

	ws := e.wsOf(id)
	if ws == nil {
		fail(fmt.Errorf("workspace missing"))
		return
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

	var summaries []string
	for i, title := range titles {
		e.update(id, func(r *Run) { r.Steps[i].Status = StepActive })
		summary, err := e.executeStep(ctx, ws, task, titles, i)
		if err != nil {
			fail(err)
			return
		}
		summaries = append(summaries, summary)
		files, _ := ws.List()
		e.update(id, func(r *Run) {
			r.Steps[i].Status = StepDone
			r.Files = files
		})
		e.log.Info("agent step done", "run_id", id, "step", i+1, "title", title, "files", len(files))
	}

	files, _ := ws.List()
	hasFiles := len(files) > 0

	e.update(id, func(r *Run) { r.Status = StatusInspecting })
	deliverable := ws.Dump(inspectDumpLimit)
	if !hasFiles {
		deliverable = strings.Join(summaries, "\n\n")
	}
	verdict, err := e.inspect(ctx, task, deliverable)
	if err != nil {
		fail(err)
		return
	}

	result := buildResult(summaries, files)
	if !inspectionPassed(verdict) {
		e.update(id, func(r *Run) { r.Status = StatusFixing })
		if hasFiles {
			if err := e.fixWithTools(ctx, ws, task, verdict); err != nil {
				fail(err)
				return
			}
			files, _ = ws.List()
			result = buildResult(summaries, files)
		} else {
			// No files were produced — fall back to A1-style text fix so
			// the run still yields something usable.
			fixed, err := e.chat(ctx, fixTextSystem, "TASK:\n"+task+"\n\nDELIVERABLE:\n"+deliverable+"\n\nINSPECTION NOTES:\n"+verdict)
			if err != nil {
				fail(err)
				return
			}
			result = fixed
		}
	}

	e.update(id, func(r *Run) {
		r.Status = StatusCompleted
		r.Result = result
		r.Files = files
	})
	e.log.Info("agent run completed", "run_id", id, "steps", len(titles), "files", len(files), "fixed", !inspectionPassed(verdict))
}

// chatMessages is one bounded multi-turn model call through the seam.
func (e *Engine) chatMessages(ctx context.Context, msgs []provider.Message) (string, error) {
	resp, err := e.provider.ChatCompletion(ctx, &provider.ChatRequest{Model: e.model, Messages: msgs})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("upstream returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// chat is a single system+user exchange.
func (e *Engine) chat(ctx context.Context, system, user string) (string, error) {
	return e.chatMessages(ctx, []provider.Message{
		{Role: provider.RoleSystem, Content: system},
		{Role: provider.RoleUser, Content: user},
	})
}

const planSystem = "You are the planning module of a build agent. Reply with ONLY a JSON array of 3 to 5 short step titles (plain strings, in the same language as the task) that break the task into concrete build steps. No prose, no code fences."

func (e *Engine) plan(ctx context.Context, task string) ([]string, error) {
	raw, err := e.chat(ctx, planSystem, task)
	if err != nil {
		return nil, err
	}
	if titles := parsePlan(raw); len(titles) > 0 {
		return titles, nil
	}
	return []string{"تنفيذ المهمة"}, nil
}

const execToolSystem = `You are the execution module of a build agent with a file workspace. You work by calling tools, one per turn. Reply with ONLY one JSON object, no prose, no code fences:
{"tool":"write_file","args":{"path":"index.html","content":"..."}}
{"tool":"read_file","args":{"path":"index.html"}}
{"tool":"edit_file","args":{"path":"index.html","find":"old","replace":"new"}}
{"tool":"list_files","args":{}}
{"tool":"search_files","args":{"query":"navbar"}}
When the current step is fully done, reply:
{"tool":"done","args":{"summary":"what you did"}}
Rules: relative paths only; produce real, complete file contents; do this step's work then call done.`

// executeStep runs the tool loop for one plan step and returns a short
// summary of what it did.
func (e *Engine) executeStep(ctx context.Context, ws *Workspace, task string, titles []string, i int) (string, error) {
	files, _ := ws.List()
	var u strings.Builder
	u.WriteString("TASK:\n" + task + "\n\nPLAN:\n")
	for n, t := range titles {
		fmt.Fprintf(&u, "%d. %s\n", n+1, t)
	}
	u.WriteString("\nFILES PRESENT:\n" + listOrNone(files))
	fmt.Fprintf(&u, "\n\nCURRENT STEP (%d): %s", i+1, titles[i])

	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: execToolSystem},
		{Role: provider.RoleUser, Content: u.String()},
	}
	for iter := 0; iter < maxToolCalls; iter++ {
		out, err := e.chatMessages(ctx, msgs)
		if err != nil {
			return "", err
		}
		tc := parseToolCall(out)
		if tc == nil {
			// No tool call — treat the text as the step's result rather
			// than failing on model formatting.
			return firstLine(out), nil
		}
		if tc.Tool == "done" {
			if s := tc.str("summary"); s != "" {
				return s, nil
			}
			return titles[i], nil
		}
		result := execTool(ws, tc)
		msgs = append(msgs,
			provider.Message{Role: provider.RoleAssistant, Content: out},
			provider.Message{Role: provider.RoleUser, Content: "TOOL RESULT:\n" + truncate(result, maxToolResult)},
		)
	}
	return titles[i], nil
}

const inspectSystem = "You are the inspection module of a build agent. Compare the deliverable to the task. If it fulfils the task, reply with exactly OK. Otherwise list the concrete problems, briefly."

func (e *Engine) inspect(ctx context.Context, task, deliverable string) (string, error) {
	return e.chat(ctx, inspectSystem, "TASK:\n"+task+"\n\nDELIVERABLE:\n"+deliverable)
}

const fixToolSystem = `You are the fixing module of a build agent with a file workspace. Fix the issues from the inspection notes by editing files. Reply with ONLY one JSON object per turn (same tools as execution: read_file, write_file, edit_file, list_files, search_files). When the issues are fixed, reply {"tool":"done","args":{"summary":"..."}}. Relative paths only.`

func (e *Engine) fixWithTools(ctx context.Context, ws *Workspace, task, notes string) error {
	files, _ := ws.List()
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: fixToolSystem},
		{Role: provider.RoleUser, Content: "TASK:\n" + task + "\n\nFILES:\n" + listOrNone(files) + "\n\nINSPECTION NOTES:\n" + notes + "\n\nFix the issues, then call done."},
	}
	for iter := 0; iter < maxToolCalls; iter++ {
		out, err := e.chatMessages(ctx, msgs)
		if err != nil {
			return err
		}
		tc := parseToolCall(out)
		if tc == nil || tc.Tool == "done" {
			return nil
		}
		result := execTool(ws, tc)
		msgs = append(msgs,
			provider.Message{Role: provider.RoleAssistant, Content: out},
			provider.Message{Role: provider.RoleUser, Content: "TOOL RESULT:\n" + truncate(result, maxToolResult)},
		)
	}
	return nil
}

const fixTextSystem = "You are the fixing module of a build agent. Given the task, the deliverable, and the inspection notes, output the corrected COMPLETE final deliverable only — no commentary."

// inspectionPassed treats only a clear leading OK as a pass.
func inspectionPassed(verdict string) bool {
	v := strings.TrimSpace(verdict)
	return v == "OK" || strings.HasPrefix(v, "OK\n") || strings.HasPrefix(v, "OK ")
}

// buildResult composes the human-facing result line from the step
// summaries and the produced file tree.
func buildResult(summaries, files []string) string {
	var b strings.Builder
	if len(files) > 0 {
		fmt.Fprintf(&b, "تم بناء %d ملف:\n", len(files))
		for _, f := range files {
			b.WriteString("• " + f + "\n")
		}
	}
	trimmed := make([]string, 0, len(summaries))
	for _, s := range summaries {
		if s = strings.TrimSpace(s); s != "" {
			trimmed = append(trimmed, "- "+s)
		}
	}
	if len(trimmed) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(strings.Join(trimmed, "\n"))
	}
	if b.Len() == 0 {
		return "تم."
	}
	return b.String()
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

func listOrNone(files []string) string {
	if len(files) == 0 {
		return "(none yet)"
	}
	return strings.Join(files, "\n")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n...[truncated]"
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
