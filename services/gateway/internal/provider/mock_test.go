package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ai-dos/foundation/util"
)

func TestMock_EchoesLastUserMessage(t *testing.T) {
	m := &Mock{Clock: util.FakeClock{FixedTime: time.Unix(1700000000, 0)}}

	resp, err := m.ChatCompletion(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: RoleSystem, Content: "be terse"},
			{Role: RoleUser, Content: "first question"},
			{Role: RoleAssistant, Content: "first answer"},
			{Role: RoleUser, Content: "second question"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Object != "chat.completion" {
		t.Errorf("object: got %q", resp.Object)
	}
	if resp.Model != "test-model" {
		t.Errorf("model: got %q", resp.Model)
	}
	if resp.Created != 1700000000 {
		t.Errorf("created: got %d", resp.Created)
	}
	if !strings.HasPrefix(resp.ID, "chatcmpl-") {
		t.Errorf("id should have chatcmpl- prefix, got %q", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices: got %d, want 1", len(resp.Choices))
	}
	choice := resp.Choices[0]
	if choice.Message.Role != RoleAssistant || choice.FinishReason != "stop" {
		t.Errorf("choice shape wrong: %+v", choice)
	}
	if !strings.Contains(choice.Message.Content, "second question") {
		t.Errorf("should echo the LAST user message, got %q", choice.Message.Content)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != resp.Usage.PromptTokens+resp.Usage.CompletionTokens {
		t.Errorf("usage should be present and internally consistent: %+v", resp.Usage)
	}
}

func TestMock_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewMock().ChatCompletion(ctx, &ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}
