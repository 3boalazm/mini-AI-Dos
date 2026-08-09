package agent

import (
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

	if got := execTool(ws, &toolCall{Tool: "write_file", Args: map[string]any{"path": "a.txt", "content": "hi"}}); !strings.HasPrefix(got, "OK") {
		t.Errorf("write: %q", got)
	}
	if got := execTool(ws, &toolCall{Tool: "read_file", Args: map[string]any{"path": "a.txt"}}); got != "hi" {
		t.Errorf("read: %q", got)
	}
	if got := execTool(ws, &toolCall{Tool: "read_file", Args: map[string]any{"path": "missing"}}); !strings.HasPrefix(got, "ERROR:") {
		t.Errorf("missing read should be an ERROR result, got %q", got)
	}
	if got := execTool(ws, &toolCall{Tool: "list_files", Args: map[string]any{}}); got != "a.txt" {
		t.Errorf("list: %q", got)
	}
	if got := execTool(ws, &toolCall{Tool: "wat", Args: map[string]any{}}); !strings.HasPrefix(got, "ERROR: unknown tool") {
		t.Errorf("unknown tool: %q", got)
	}
	// A traversal path comes back as a contained ERROR, not a panic.
	if got := execTool(ws, &toolCall{Tool: "write_file", Args: map[string]any{"path": "../escape", "content": "x"}}); !strings.HasPrefix(got, "ERROR:") {
		t.Errorf("traversal write should be an ERROR result, got %q", got)
	}
}
