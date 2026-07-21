package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_JSONInProduction(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Environment: EnvProduction, Output: &buf})

	log.Info("hello", "key", "value")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON output in production mode, got error: %v\noutput: %s", err, buf.String())
	}
	if parsed["msg"] != "hello" {
		t.Errorf("expected msg=hello, got %v", parsed["msg"])
	}
	if parsed["key"] != "value" {
		t.Errorf("expected key=value, got %v", parsed["key"])
	}
}

func TestNew_TextInDevelopment(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Environment: EnvDevelopment, Output: &buf})

	log.Info("hello")

	if json.Valid(buf.Bytes()) {
		t.Errorf("expected human-readable text in development mode, got JSON: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected output to contain the log message, got: %s", buf.String())
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Environment: EnvProduction, Level: "warn", Output: &buf})

	log.Info("should be filtered out")
	if buf.Len() != 0 {
		t.Errorf("expected info-level log to be filtered when level=warn, got output: %s", buf.String())
	}

	log.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("expected warn-level log to appear when level=warn, got no output")
	}
}

func TestFromContext_AttachesTraceID(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Environment: EnvProduction, Output: &buf})

	ctx := WithTraceID(context.Background(), "trace-abc-123")
	scoped := log.FromContext(ctx)
	scoped.Info("request handled")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if parsed["trace_id"] != "trace-abc-123" {
		t.Errorf("expected trace_id=trace-abc-123 to be attached, got %v", parsed["trace_id"])
	}
}

func TestFromContext_NoTraceIDIsSafe(t *testing.T) {
	var buf bytes.Buffer
	log := New(Config{Environment: EnvProduction, Output: &buf})

	// No trace ID in context — must not panic, must not add an empty field.
	scoped := log.FromContext(context.Background())
	scoped.Info("no trace context")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if _, present := parsed["trace_id"]; present {
		t.Errorf("expected no trace_id field when context carries none, got %v", parsed["trace_id"])
	}
}
