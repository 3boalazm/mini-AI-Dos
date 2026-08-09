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
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	apperrors "github.com/ai-dos/foundation/errors"
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
	// maxLogEntries caps the per-run tool-activity log kept for the UI.
	maxLogEntries = 40
)

// rateBackoff is the wait schedule between retries when the upstream
// returns a rate-limit error — the agent bursts many calls and would
// otherwise die on a free-tier per-minute cap. Each entry is one more
// retry; the list length is the retry count. Overridable per engine
// for fast tests.
var rateBackoff = []time.Duration{5 * time.Second, 12 * time.Second, 20 * time.Second, 30 * time.Second}

// Status is the run lifecycle. The progression is fixed:
// planning → executing → inspecting → (fixing) → completed | failed.
type Status string

const (
	StatusPlanning         Status = "planning"
	StatusPlanned          Status = "planned" // plan ready, awaiting user approval (A7)
	StatusExecuting        Status = "executing"
	StatusInspecting       Status = "inspecting"
	StatusFixing           Status = "fixing"
	StatusAwaitingApproval Status = "awaiting_approval" // a sensitive command is waiting for a decision (A8)
	StatusCompleted        Status = "completed"
	StatusFailed           Status = "failed"
)

// Approval describes a sensitive action the run is paused on, shown to
// the user so they can Allow/Deny it (A8).
type Approval struct {
	Tool    string `json:"tool"`
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

// approvalDecision is the user's answer to a pending Approval.
type approvalDecision struct {
	allow    bool
	remember bool // "always allow" this category for the rest of the run
}

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
	ID        string       `json:"id"`
	Task      string       `json:"task"`
	Status    Status       `json:"status"`
	Steps     []Step       `json:"steps"`
	Files     []string     `json:"files"`
	Log       []string     `json:"log,omitempty"`
	Pending   *Approval    `json:"pending,omitempty"`
	CmdErrors int          `json:"cmd_errors,omitempty"`
	Recovered bool         `json:"recovered,omitempty"`
	Changes   []FileChange `json:"changes,omitempty"`
	Result    string       `json:"result,omitempty"`
	Error     string       `json:"error,omitempty"`
	Created   int64        `json:"created"`

	cancel        context.CancelFunc
	ws            *Workspace
	approve       chan struct{}         // signalled by Approve() to release a planned run (A7)
	decision      chan approvalDecision // carries the user's Allow/Deny for a pending command (A8)
	allowed       map[string]bool       // categories the user chose to "always allow" this run (A8)
	hadCmdFailure bool                  // a run_command has failed at least once (A9)
	buildSnap     map[string]string     // workspace snapshot after the build, before fixes (A10 revert/compare)
}

// Engine owns every run and drives the loop. One engine per server.
type Engine struct {
	provider provider.Provider
	model    string
	baseDir  string
	log      *logging.Logger
	backoff  []time.Duration

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
	return &Engine{provider: p, model: model, baseDir: baseDir, log: log, backoff: rateBackoff, runs: make(map[string]*Run)}
}

// Start validates and launches a run, returning its first snapshot.
// When autoStart is false, the run stops after planning at status
// "planned" until Approve is called (A7); when true it runs straight
// through — the default for direct API callers.
func (e *Engine) Start(task string, autoStart bool) (*Run, error) {
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
		ID:       id,
		Task:     task,
		Status:   StatusPlanning,
		Created:  time.Now().Unix(),
		cancel:   cancel,
		ws:       ws,
		approve:  make(chan struct{}, 1),
		decision: make(chan approvalDecision, 1),
		allowed:  make(map[string]bool),
	}
	e.mu.Lock()
	e.runs[id] = run
	e.evictLocked()
	e.mu.Unlock()

	go e.loop(ctx, id, task, autoStart)
	return e.Get(id), nil
}

// Approve releases a run waiting at the "planned" gate. Reports whether
// the run was known and awaiting approval.
func (e *Engine) Approve(id string) bool {
	e.mu.RLock()
	r, ok := e.runs[id]
	awaiting := ok && r.Status == StatusPlanned
	ch := (chan struct{})(nil)
	if ok {
		ch = r.approve
	}
	e.mu.RUnlock()
	if !awaiting || ch == nil {
		return false
	}
	select {
	case ch <- struct{}{}:
		return true
	default:
		return false // already approved
	}
}

// Decide answers a run's pending command approval (A8). remember="always
// allow" whitelists the category for the rest of the run. Reports
// whether the run was actually awaiting a decision.
func (e *Engine) Decide(id string, allow, remember bool) bool {
	e.mu.RLock()
	r, ok := e.runs[id]
	awaiting := ok && r.Status == StatusAwaitingApproval
	var ch chan approvalDecision
	if ok {
		ch = r.decision
	}
	e.mu.RUnlock()
	if !awaiting || ch == nil {
		return false
	}
	select {
	case ch <- approvalDecision{allow: allow, remember: remember}:
		return true
	default:
		return false
	}
}

// runTool dispatches one tool, routing a sensitive run_command through
// the approval gate first (A8). A denied command returns a result the
// model reads and adapts to rather than a hard failure.
func (e *Engine) runTool(ctx context.Context, id string, ws *Workspace, tc *toolCall) string {
	if tc.Tool == "run_command" {
		if reason, sensitive := commandCategory(tc.str("command")); sensitive {
			if !e.requestApproval(ctx, id, tc.str("command"), reason) {
				return "DENIED by the user — do not retry this command; find another approach or finish. Command: " + tc.str("command")
			}
		}
	}
	result := execTool(ctx, ws, tc)
	if tc.Tool == "run_command" {
		e.recordCommandOutcome(id, result)
	}
	return result
}

// recordCommandOutcome tracks A9 error recovery: a command that exits
// non-zero (or times out) is a failure; a later successful command
// after any failure marks the run as recovered, which the final report
// surfaces succinctly instead of dumping raw errors.
func (e *Engine) recordCommandOutcome(id, result string) {
	e.update(id, func(r *Run) {
		switch {
		case commandFailed(result):
			r.CmdErrors++
			r.hadCmdFailure = true
		case commandSucceeded(result) && r.hadCmdFailure:
			r.Recovered = true
		}
	})
}

func commandFailed(result string) bool {
	return strings.HasPrefix(result, "EXIT ") || strings.HasPrefix(result, "ERROR: command timed out")
}

func commandSucceeded(result string) bool {
	return strings.HasPrefix(result, "OK (exit 0")
}

// requestApproval pauses the run on a sensitive command until the user
// decides, unless the category was already "always allowed". Returns
// whether the command may run. A cancelled context denies.
func (e *Engine) requestApproval(ctx context.Context, id, command, reason string) bool {
	e.mu.RLock()
	r, ok := e.runs[id]
	pre := ok && r.allowed[reason]
	var ch chan approvalDecision
	if ok {
		ch = r.decision
	}
	e.mu.RUnlock()
	if pre {
		return true
	}

	var prev Status
	e.update(id, func(r *Run) {
		prev = r.Status
		r.Status = StatusAwaitingApproval
		r.Pending = &Approval{Tool: "run_command", Command: command, Reason: reason}
	})
	e.log.Info("agent awaiting command approval", "run_id", id, "reason", reason, "command", command)

	select {
	case d := <-ch:
		e.update(id, func(r *Run) {
			r.Pending = nil
			r.Status = prev
			if d.allow && d.remember {
				r.allowed[reason] = true
			}
		})
		return d.allow
	case <-ctx.Done():
		e.update(id, func(r *Run) { r.Pending = nil; r.Status = prev })
		return false
	}
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
	cp.approve = nil
	cp.decision = nil
	cp.allowed = nil
	cp.buildSnap = nil
	cp.Steps = append([]Step(nil), r.Steps...)
	cp.Files = append([]string(nil), r.Files...)
	cp.Log = append([]string(nil), r.Log...)
	cp.Changes = append([]FileChange(nil), r.Changes...)
	if r.Pending != nil {
		p := *r.Pending
		cp.Pending = &p
	}
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

// Compare returns a unified diff of what changed since the build
// snapshot (A10). known reports whether the run id exists.
func (e *Engine) Compare(id string) (diff string, known bool) {
	e.mu.RLock()
	r, ok := e.runs[id]
	var before map[string]string
	var ws *Workspace
	if ok {
		before, ws = r.buildSnap, r.ws
	}
	e.mu.RUnlock()
	if !ok || ws == nil {
		return "", false
	}
	if before == nil {
		return "(no build snapshot yet)", true
	}
	return unifiedDiff(before, ws.Snapshot()), true
}

// Revert restores a run's workspace to its post-build snapshot, undoing
// the fix stage's changes (A10). Reports whether it was applied.
func (e *Engine) Revert(id string) bool {
	e.mu.RLock()
	r, ok := e.runs[id]
	var before map[string]string
	var ws *Workspace
	if ok {
		before, ws = r.buildSnap, r.ws
	}
	e.mu.RUnlock()
	if !ok || ws == nil || before == nil {
		return false
	}
	if err := ws.Restore(before); err != nil {
		e.log.Warn("revert failed", "run_id", id, "error", err.Error())
		return false
	}
	files, _ := ws.List()
	e.update(id, func(r *Run) {
		r.Files = files
		r.Changes = nil
	})
	return true
}

// ZipRun writes every file in a run's workspace into w as a zip
// archive. known reports whether the run id exists, so a missing run
// and an empty workspace are distinguishable at the HTTP layer.
func (e *Engine) ZipRun(id string, w io.Writer) (known bool, err error) {
	e.mu.RLock()
	r, ok := e.runs[id]
	e.mu.RUnlock()
	if !ok || r.ws == nil {
		return false, nil
	}
	return true, r.ws.WriteZip(w)
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

// evictLocked removes the oldest *finished* run (and its workspace) once
// the cap is exceeded. In-flight runs are never evicted — a live run
// must not vanish out from under the user just because newer runs were
// created. Caller holds the write lock.
func (e *Engine) evictLocked() {
	if len(e.runs) <= maxRuns {
		return
	}
	var oldestID string
	var oldest int64
	first := true
	for id, r := range e.runs {
		if !isTerminal(r.Status) {
			continue // only reclaim completed/failed runs
		}
		if first || r.Created < oldest {
			oldest, oldestID, first = r.Created, id, false
		}
	}
	if oldestID == "" {
		return // nothing terminal to reclaim yet; let the map grow a little
	}
	if r, ok := e.runs[oldestID]; ok {
		if r.ws != nil {
			_ = r.ws.Remove()
		}
		delete(e.runs, oldestID)
	}
}

// isTerminal reports whether a run has reached a final status.
func isTerminal(s Status) bool {
	return s == StatusCompleted || s == StatusFailed
}

// update applies fn to a run under the write lock.
func (e *Engine) update(id string, fn func(*Run)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.runs[id]; ok {
		fn(r)
	}
}

// appendLog records one tool action on the run's activity log, capped
// so a long run doesn't grow it without bound.
func (e *Engine) appendLog(id, line string) {
	e.update(id, func(r *Run) {
		r.Log = append(r.Log, line)
		if len(r.Log) > maxLogEntries {
			r.Log = r.Log[len(r.Log)-maxLogEntries:]
		}
	})
}

// toolActivity summarizes a tool call for the activity log — verb plus
// its most telling argument, never full file contents.
func toolActivity(tc *toolCall) string {
	switch tc.Tool {
	case "run_command":
		return "$ " + truncate(tc.str("command"), 80)
	case "read_file", "write_file", "edit_file", "inspect_page":
		return tc.Tool + ": " + tc.str("path")
	case "search_files":
		return "search: " + tc.str("query")
	case "list_files":
		return "list_files"
	default:
		return tc.Tool
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

// approveChan returns a run's approval channel (set once at Start).
func (e *Engine) approveChan(id string) chan struct{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if r, ok := e.runs[id]; ok {
		return r.approve
	}
	return nil
}

// loop is the whole agent: plan, (optionally wait for approval,) execute
// each step with tools, inspect the workspace, fix once if inspection
// objects.
func (e *Engine) loop(ctx context.Context, id, task string, autoStart bool) {
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

	// A7: show the plan and wait for approval before doing any work.
	if !autoStart {
		e.update(id, func(r *Run) { r.Steps = steps; r.Status = StatusPlanned })
		ch := e.approveChan(id)
		select {
		case <-ch:
			// approved — fall through to execution
		case <-ctx.Done():
			fail(fmt.Errorf("cancelled before approval"))
			return
		}
	}
	e.update(id, func(r *Run) { r.Steps = steps; r.Status = StatusExecuting })

	var summaries []string
	for i, title := range titles {
		e.update(id, func(r *Run) { r.Steps[i].Status = StepActive })
		summary, err := e.executeStep(ctx, id, ws, task, titles, i)
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

	// A10: snapshot the post-build workspace, so any changes the fix
	// stage makes can be summarized, compared, and reverted.
	buildSnap := ws.Snapshot()
	e.update(id, func(r *Run) { r.buildSnap = buildSnap })

	e.update(id, func(r *Run) { r.Status = StatusInspecting })
	deliverable := ws.Dump(inspectDumpLimit)
	if !hasFiles {
		deliverable = strings.Join(summaries, "\n\n")
	}
	// A4: ground the model's inspection in automated structural checks of
	// the HTML it produced (broken links, missing assets, missing alt),
	// so "See → Critique" catches concrete defects, not just vibes.
	autoChecks := ws.InspectAllHTML()
	verdict, err := e.inspect(ctx, task, deliverable, autoChecks)
	if err != nil {
		fail(err)
		return
	}

	result := buildResult(summaries, files)
	if !inspectionPassed(verdict) {
		e.update(id, func(r *Run) { r.Status = StatusFixing })
		if hasFiles {
			if err := e.fixWithTools(ctx, id, ws, task, verdict); err != nil {
				fail(err)
				return
			}
			files, _ = ws.List()
			result = buildResult(summaries, files)
			// A10: record what the fix stage changed vs the build.
			changes := diffSnapshots(buildSnap, ws.Snapshot())
			e.update(id, func(r *Run) { r.Changes = changes })
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
		// A9: report recovery as a one-line summary, not a raw error dump.
		if r.Recovered {
			r.Result = "⚠ واجه خطأ أثناء التنفيذ وتعافى منه تلقائيًا.\n\n" + result
		} else {
			r.Result = result
		}
		r.Files = files
	})
	e.log.Info("agent run completed", "run_id", id, "steps", len(titles), "files", len(files), "fixed", !inspectionPassed(verdict))
}

// chatMessages is one model call, retried with backoff on upstream
// rate-limit errors — a run bursts many calls and would otherwise die
// on a free-tier per-minute cap. Backoff waits respect ctx, so a
// cancelled run stops waiting immediately.
func (e *Engine) chatMessages(ctx context.Context, msgs []provider.Message) (string, error) {
	for attempt := 0; ; attempt++ {
		out, err := e.chatOnce(ctx, msgs)
		if err == nil || !isRateLimited(err) || attempt >= len(e.backoff) {
			return out, err
		}
		wait := e.backoff[attempt]
		e.log.Info("agent backing off on upstream rate limit", "attempt", attempt+1, "wait", wait.String())
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(wait):
		}
	}
}

// chatOnce is a single multi-turn model call through the seam.
func (e *Engine) chatOnce(ctx context.Context, msgs []provider.Message) (string, error) {
	resp, err := e.provider.ChatCompletion(ctx, &provider.ChatRequest{Model: e.model, Messages: msgs})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("upstream returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// isRateLimited reports whether err is (or wraps) an upstream
// rate-limit AppError.
func isRateLimited(err error) bool {
	var ae *apperrors.AppError
	if stderrors.As(err, &ae) {
		return ae.Code == apperrors.CodeRateLimited
	}
	return false
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

const execToolSystem = `You are the execution module of a build agent with a file workspace and a shell. You work by calling tools, one per turn. Reply with ONLY one JSON object, no prose, no code fences:
{"tool":"write_file","args":{"path":"index.html","content":"..."}}
{"tool":"read_file","args":{"path":"index.html"}}
{"tool":"edit_file","args":{"path":"index.html","find":"old","replace":"new"}}
{"tool":"list_files","args":{}}
{"tool":"search_files","args":{"query":"navbar"}}
{"tool":"run_command","args":{"command":"ls -la"}}
{"tool":"inspect_page","args":{"path":"index.html"}}
When the current step is fully done, reply:
{"tool":"done","args":{"summary":"what you did"}}
Rules: relative paths only; produce real, complete file contents; run_command runs in the workspace; inspect_page checks an HTML file for broken links, missing assets, and missing alt/meta — use it to verify a page you built. If a run_command fails (non-zero EXIT or an ERROR result), read the output, fix the cause, and run it again — don't give up after one failure. Do this step's work then call done.`

// executeStep runs the tool loop for one plan step and returns a short
// summary of what it did.
func (e *Engine) executeStep(ctx context.Context, id string, ws *Workspace, task string, titles []string, i int) (string, error) {
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
			// No parseable tool call — treat the text as the step's
			// summary, but never leak a raw/broken tool-call JSON (a
			// model can emit invalid JSON that fails to parse) into the
			// user-facing result; fall back to the step title then.
			return cleanSummary(firstLine(out), titles[i]), nil
		}
		if tc.Tool == "done" {
			return cleanSummary(tc.str("summary"), titles[i]), nil
		}
		e.appendLog(id, toolActivity(tc))
		result := e.runTool(ctx, id, ws, tc)
		msgs = append(msgs,
			provider.Message{Role: provider.RoleAssistant, Content: out},
			provider.Message{Role: provider.RoleUser, Content: "TOOL RESULT:\n" + truncate(result, maxToolResult)},
		)
	}
	return titles[i], nil
}

const inspectSystem = "You are the inspection module of a build agent. Compare the deliverable to the task, and treat the AUTOMATED CHECKS (if any) as verified facts about the built pages. If the deliverable fulfils the task and the automated checks report no real problems, reply with exactly OK. Otherwise list the concrete problems to fix, briefly."

func (e *Engine) inspect(ctx context.Context, task, deliverable, autoChecks string) (string, error) {
	user := "TASK:\n" + task + "\n\nDELIVERABLE:\n" + deliverable
	if strings.TrimSpace(autoChecks) != "" {
		user += "\n\nAUTOMATED CHECKS:\n" + autoChecks
	}
	return e.chat(ctx, inspectSystem, user)
}

const fixToolSystem = `You are the fixing module of a build agent with a file workspace and a shell. Fix the issues from the inspection notes by editing files. Reply with ONLY one JSON object per turn (same tools as execution: read_file, write_file, edit_file, list_files, search_files, run_command, inspect_page). Use inspect_page to confirm an HTML fix resolved the reported issue. If a run_command fails, read the error, fix the cause, and re-run it before giving up. When the issues are fixed, reply {"tool":"done","args":{"summary":"..."}}. Relative paths only.`

func (e *Engine) fixWithTools(ctx context.Context, id string, ws *Workspace, task, notes string) error {
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
		e.appendLog(id, toolActivity(tc))
		result := e.runTool(ctx, id, ws, tc)
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

// cleanSummary returns s unless it is empty or looks like raw tool
// protocol (a JSON object or a "tool" field a model emitted instead of
// a plain summary), in which case it returns the fallback. This keeps
// the internal tool protocol out of the user-facing result.
func cleanSummary(s, fallback string) string {
	t := strings.TrimSpace(s)
	if t == "" || strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") || strings.Contains(t, `"tool"`) {
		return fallback
	}
	return t
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
