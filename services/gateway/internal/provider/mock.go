package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/foundation/util"
)

// Mock is a deterministic in-process provider: it echoes the last user
// message. It exists so the gateway is runnable and testable with no
// upstream credentials at all (AI_PROVIDER=mock, the default), and so
// the E2E test can assert the full request path without network access.
type Mock struct {
	Clock util.Clock
}

// NewMock returns a Mock using the real clock.
func NewMock() *Mock {
	return &Mock{Clock: util.RealClock{}}
}

func (m *Mock) Name() string { return "mock" }

// ChatCompletion echoes the last user message. Usage is real counting
// performed by this provider (it IS the upstream here), not a
// fabricated stand-in for a remote provider's numbers.
func (m *Mock) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(errors.CodeTimeout, "request canceled before completion", err)
	}

	lastUser := ""
	for _, msg := range req.Messages {
		if msg.Role == RoleUser {
			lastUser = msg.Content
		}
	}
	content := fmt.Sprintf("mock response to: %s", lastUser)

	id, err := util.NewUUIDv7()
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "failed to generate completion id", err)
	}

	promptWords := 0
	for _, msg := range req.Messages {
		promptWords += len(strings.Fields(msg.Content))
	}
	completionWords := len(strings.Fields(content))

	return &ChatResponse{
		ID:      "chatcmpl-" + id,
		Object:  "chat.completion",
		Created: m.Clock.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{{
			Index:        0,
			Message:      Message{Role: RoleAssistant, Content: content},
			FinishReason: "stop",
		}},
		Usage: &Usage{
			PromptTokens:     promptWords,
			CompletionTokens: completionWords,
			TotalTokens:      promptWords + completionWords,
		},
	}, nil
}
