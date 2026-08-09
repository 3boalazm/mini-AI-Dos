package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ai-dos/foundation/errors"
)

// agentCreateRequest is the POST /v1/agent/runs body.
type agentCreateRequest struct {
	Task string `json:"task"`
}

// handleAgentCreate is POST /v1/agent/runs — starts an agent run and
// returns its first snapshot. The heavy work happens asynchronously;
// callers poll the run id.
func (s *Server) handleAgentCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, errors.New(errors.CodeValidation, "method not allowed: use POST"))
		return
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeError(w, errors.New(errors.CodeValidation, "Content-Type must be application/json"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	var req agentCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.Wrap(errors.CodeValidation, "request body is not valid JSON", err))
		return
	}

	run, err := s.agent.Start(req.Task)
	if err != nil {
		writeError(w, errors.Wrap(errors.CodeValidation, "cannot start agent run", err))
		return
	}

	if meta := metaFromContext(r.Context()); meta != nil {
		meta.Provider = s.provider.Name()
		meta.Model = "agent:" + run.ID
	}
	writeJSON(w, http.StatusAccepted, run)
}

// handleAgentRun serves the per-run routes under /v1/agent/runs/{id}:
// POST .../cancel (real cancellation), GET .../files/{path} (fetch one
// workspace file), and GET .../{id} (poll snapshot).
func (s *Server) handleAgentRun(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/agent/runs/")

	if id, ok := strings.CutSuffix(rest, "/cancel"); ok {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, errors.New(errors.CodeValidation, "method not allowed: use POST"))
			return
		}
		if !s.agent.Cancel(id) {
			writeError(w, errors.New(errors.CodeNotFound, "no such agent run"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": "cancelling"})
		return
	}

	if idx := strings.Index(rest, "/files/"); idx >= 0 {
		s.serveRunFile(w, r, rest[:idx], rest[idx+len("/files/"):])
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, errors.New(errors.CodeValidation, "method not allowed: use GET"))
		return
	}
	run := s.agent.Get(rest)
	if run == nil {
		writeError(w, errors.New(errors.CodeNotFound, "no such agent run"))
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// serveRunFile returns one file from a run's workspace as text/plain.
// It is served with nosniff and never as HTML, so generated markup
// cannot execute in the gateway's own origin — the UI previews it in a
// sandboxed iframe instead.
func (s *Server) serveRunFile(w http.ResponseWriter, r *http.Request, id, path string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, errors.New(errors.CodeValidation, "method not allowed: use GET"))
		return
	}
	content, known, err := s.agent.ReadRunFile(id, path)
	if !known {
		writeError(w, errors.New(errors.CodeNotFound, "no such agent run"))
		return
	}
	if err != nil {
		writeError(w, errors.New(errors.CodeNotFound, err.Error()))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}
