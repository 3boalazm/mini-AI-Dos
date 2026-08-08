package server

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/gateway/internal/provider"
)

const (
	// maxRequestBytes bounds the request body — chat requests are small;
	// anything past 1 MiB is either abuse or a mistake.
	maxRequestBytes = 1 << 20
	// completionTimeout bounds one completion end to end, including the
	// upstream call. The provider's own HTTP client timeout is shorter,
	// so this is the backstop, not the primary limit.
	completionTimeout = 90 * time.Second
)

// chatRequest is the HTTP-boundary request shape. Stream is decoded
// only to reject it explicitly — streaming is a documented
// non-feature, and silently ignoring it would corrupt clients that
// expect SSE.
type chatRequest struct {
	Model       string             `json:"model"`
	Messages    []provider.Message `json:"messages"`
	Temperature *float64           `json:"temperature"`
	MaxTokens   *int               `json:"max_tokens"`
	Stream      bool               `json:"stream"`
}

// handleHealth is the unauthenticated liveness endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, errors.New(errors.CodeValidation, "method not allowed: use GET"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"provider": s.provider.Name(),
	})
}

// handleChatCompletions is POST /v1/chat/completions.
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, errors.New(errors.CodeValidation, "method not allowed: use POST"))
		return
	}

	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, openAIError{Error: openAIErrorBody{
			Message: "Content-Type must be application/json",
			Type:    "invalid_request_error",
			Code:    "unsupported_media_type",
		}})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if stderrors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, openAIError{Error: openAIErrorBody{
				Message: fmt.Sprintf("request body exceeds the %d byte limit", int64(maxRequestBytes)),
				Type:    "invalid_request_error",
				Code:    "request_too_large",
			}})
			return
		}
		writeError(w, errors.Wrap(errors.CodeValidation, "request body is not valid JSON", err))
		return
	}

	pReq, err := s.validateChatRequest(&req)
	if err != nil {
		writeError(w, err)
		return
	}

	// Record provider/model for the access log before calling upstream,
	// so even failed calls are attributed.
	if meta := metaFromContext(r.Context()); meta != nil {
		meta.Provider = s.provider.Name()
		meta.Model = pReq.Model
	}

	ctx, cancel := s.completionContext(r)
	defer cancel()

	resp, err := s.provider.ChatCompletion(ctx, pReq)
	if err != nil {
		// A backstop-timeout expiry must surface as 504 even if the
		// provider wrapped it differently.
		if ctx.Err() != nil && asAppError(err).Code != errors.CodeTimeout {
			err = errors.Wrap(errors.CodeTimeout, "request timed out", err)
		}
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// validateChatRequest enforces the request contract and normalizes into
// the provider-layer type. Every rejection is a CodeValidation error
// with a message specific enough to fix the request.
func (s *Server) validateChatRequest(req *chatRequest) (*provider.ChatRequest, error) {
	if req.Stream {
		return nil, errors.New(errors.CodeValidation, "streaming is not supported by Mini AI-DOS; set stream to false or omit it")
	}

	model := req.Model
	if model == "" {
		model = s.cfg.AIModel
	}
	if model == "" {
		return nil, errors.New(errors.CodeValidation, "model is required (no AI_MODEL default is configured)")
	}

	if len(req.Messages) == 0 {
		return nil, errors.New(errors.CodeValidation, "messages must be a non-empty array")
	}
	for i, m := range req.Messages {
		switch m.Role {
		case provider.RoleSystem, provider.RoleUser, provider.RoleAssistant:
		default:
			return nil, errors.New(errors.CodeValidation,
				fmt.Sprintf("messages[%d].role must be one of system, user, assistant", i))
		}
		if m.Content == "" {
			return nil, errors.New(errors.CodeValidation,
				fmt.Sprintf("messages[%d].content must be a non-empty string (multi-part content is not supported)", i))
		}
	}

	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return nil, errors.New(errors.CodeValidation, "temperature must be between 0 and 2")
	}
	if req.MaxTokens != nil && *req.MaxTokens <= 0 {
		return nil, errors.New(errors.CodeValidation, "max_tokens must be a positive integer")
	}

	return &provider.ChatRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}, nil
}
