package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// A2 file tools. The provider seam speaks plain chat — no native
// tool-calling wire format — so tools use a text JSON protocol: the
// model emits one JSON object per turn, we execute it and feed the
// result back. This is deliberately provider-agnostic; it works
// identically across Gemini, Groq, and every other backend in the
// stack, none of which share a tool-calling format.
//
// Deliberately excluded until later phases: delete, install, git.

// toolCall is one parsed tool invocation from model output.
type toolCall struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// str reads a string argument, empty when absent or the wrong type.
func (tc *toolCall) str(key string) string {
	if tc.Args == nil {
		return ""
	}
	if v, ok := tc.Args[key].(string); ok {
		return v
	}
	return ""
}

// parseToolCall extracts the first JSON object from raw model output,
// tolerating prose or fences around it. Returns nil when there is no
// usable tool call — the caller then treats the text as a direct
// answer rather than failing.
func parseToolCall(raw string) *toolCall {
	start := strings.Index(raw, "{")
	if start < 0 {
		return nil
	}
	// A decoder reads exactly one JSON value and ignores trailing
	// text, so a model that appends commentary after the object still
	// parses.
	dec := json.NewDecoder(strings.NewReader(raw[start:]))
	var tc toolCall
	if err := dec.Decode(&tc); err != nil {
		return nil
	}
	if tc.Tool == "" {
		return nil
	}
	return &tc
}

// execTool runs one tool against the workspace and returns a result
// string. Tool-level failures come back as "ERROR: ..." results, not
// Go errors: the model reads them and can recover on the next turn.
func execTool(ctx context.Context, ws *Workspace, tc *toolCall) string {
	switch tc.Tool {
	case "inspect_page":
		report, err := ws.InspectPage(tc.str("path"))
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return report
	case "run_command":
		return ws.RunCommand(ctx, tc.str("command"))
	case "read_file":
		c, err := ws.ReadFile(tc.str("path"))
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return c
	case "write_file":
		if err := ws.WriteFile(tc.str("path"), tc.str("content")); err != nil {
			return "ERROR: " + err.Error()
		}
		return "OK: wrote " + tc.str("path")
	case "edit_file":
		n, err := ws.EditFile(tc.str("path"), tc.str("find"), tc.str("replace"))
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return fmt.Sprintf("OK: replaced %d occurrence(s) in %s", n, tc.str("path"))
	case "list_files":
		files, _ := ws.List()
		if len(files) == 0 {
			return "(empty workspace)"
		}
		return strings.Join(files, "\n")
	case "search_files":
		hits, err := ws.Search(tc.str("query"))
		if err != nil {
			return "ERROR: " + err.Error()
		}
		if len(hits) == 0 {
			return "(no matches)"
		}
		return strings.Join(hits, "\n")
	default:
		return "ERROR: unknown tool " + tc.Tool
	}
}
