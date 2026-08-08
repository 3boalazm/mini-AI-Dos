// Package provider defines the provider abstraction the gateway routes
// completions through, plus the wire types shared between the HTTP
// layer and every provider implementation. The shapes deliberately
// mirror the OpenAI Chat Completions contract — that is the
// compatibility target, per specs/contracts/gateway-contract.md.
package provider

import "context"

// Role values accepted in a chat message.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is one chat turn. Content is a plain string — multi-part
// content arrays are not supported by Mini AI-DOS (documented
// limitation, rejected with a 400 at the HTTP boundary).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the normalized completion request handed to a
// provider. Validation happens at the HTTP boundary before a provider
// ever sees the request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// Usage is token accounting as reported by the upstream provider.
// Never fabricated: if the upstream does not report usage, the
// response simply omits it.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Choice is one completion alternative in a response.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatResponse is the normalized completion response in the
// OpenAI-compatible shape the gateway returns to callers.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Provider is the whole abstraction: a name for logs and one call.
// Errors returned by ChatCompletion should be *errors.AppError from
// the foundation package so the HTTP layer can map them to statuses
// without provider-specific knowledge.
type Provider interface {
	Name() string
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}
