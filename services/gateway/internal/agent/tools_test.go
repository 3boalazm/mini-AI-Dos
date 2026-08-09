package agent

import (
	"context"
	"strings"
	"testing"
)

func TestParseToolCall(t *testing.T) {
	if tc := parseToolCall(`{"tool":"list_files","args":{}}`); tc == nil || tc.Tool != "list_files" {
		t.Errorf("plain object: %+v", tc)
	}
	// Prose and a code fence around the object are tolerated.
	if tc := parseToolCall("Sure:\n```json\n{\"tool\":\"read_file\",\"args\":{\"path\":\"a.txt\"}}\n```"); tc == nil || tc.str("path") != "a.txt" {
		t.Errorf("wrapped object not parsed: %+v", tc)
	}
	// Trailing commentary after the object is ignored.
	if tc := parseToolCall(`{"tool":"done","args":{"summary":"ok"}} and that's it`); tc == nil || tc.Tool != "done" {
		t.Errorf("trailing text: %+v", tc)
	}
	if tc := parseToolCall("no json here"); tc != nil {
		t.Errorf("non-json should be nil, got %+v", tc)
	}
	if tc := parseToolCall(`{"args":{}}`); tc != nil {
		t.Error("object without a tool name should be nil")
	}
}

func TestExecTool(t *testing.T) {
	ws := newTestWS(t)
	ctx := context.Background()

	if got := execTool(ctx, ws, &toolCall{Tool: "write_file", Args: map[string]any{"path": "a.txt", "content": "hi"}}); !strings.HasPrefix(got, "OK") {
		t.Errorf("write: %q", got)
	}
	if got := execTool(ctx, ws, &toolCall{Tool: "read_file", Args: map[string]any{"path": "a.txt"}}); got != "hi" {
		t.Errorf("read: %q", got)
	}
	if got := execTool(ctx, ws, &toolCall{Tool: "read_file", Args: map[string]any{"path": "missing"}}); !strings.HasPrefix(got, "ERROR:") {
		t.Errorf("missing read should be an ERROR result, got %q", got)
	}
	if got := execTool(ctx, ws, &toolCall{Tool: "list_files", Args: map[string]any{}}); got != "a.txt" {
		t.Errorf("list: %q", got)
	}
	if got := execTool(ctx, ws, &toolCall{Tool: "wat", Args: map[string]any{}}); !strings.HasPrefix(got, "ERROR: unknown tool") {
		t.Errorf("unknown tool: %q", got)
	}
	// A traversal path comes back as a contained ERROR, not a panic.
	if got := execTool(ctx, ws, &toolCall{Tool: "write_file", Args: map[string]any{"path": "../escape", "content": "x"}}); !strings.HasPrefix(got, "ERROR:") {
		t.Errorf("traversal write should be an ERROR result, got %q", got)
	}
}

func TestRunCommand(t *testing.T) {
	ws := newTestWS(t)
	ctx := context.Background()

	// A benign command runs and its output comes back. echo exists on
	// both sh (Linux/CI) and cmd (Windows dev).
	out := execTool(ctx, ws, &toolCall{Tool: "run_command", Args: map[string]any{"command": "echo hello123"}})
	if !strings.Contains(out, "hello123") {
		t.Errorf("run_command echo: %q", out)
	}

	// Sensitive categories are refused until the approval system (A8).
	for _, c := range []string{"rm -rf .", "git status", "npm install left-pad", "sudo ls", "curl http://x"} {
		got := execTool(ctx, ws, &toolCall{Tool: "run_command", Args: map[string]any{"command": c}})
		if !strings.Contains(got, "require approval") {
			t.Errorf("command %q should be blocked, got %q", c, got)
		}
	}

	// "npm run build" is a script run, not an install — allowed through
	// the guard (it fails only because npm isn't present, which is a
	// normal EXIT result, not a block).
	if got, blocked := commandIsBlocked("npm run build"); blocked {
		t.Errorf("npm run build should not be blocked, got reason %q", got)
	}
}
