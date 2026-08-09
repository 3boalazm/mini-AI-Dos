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

// handleAgentRun serves GET /v1/agent/runs/{id} (poll snapshot) and
// POST /v1/agent/runs/{id}/cancel (real cancellation).
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
