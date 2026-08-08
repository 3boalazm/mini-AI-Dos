package server

import (
	"encoding/json"
	"net/http"

	"github.com/ai-dos/foundation/errors"
)

// openAIError is the error body shape for /v1/* routes — the
// OpenAI-compatible surface, per specs/contracts/gateway-contract.md:
// gateway completion routes speak the OpenAI error dialect, so existing
// OpenAI clients understand failures without special-casing.
type openAIError struct {
	Error openAIErrorBody `json:"error"`
}

type openAIErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// errorTypeByCode maps the foundation's error codes onto OpenAI error
// type strings. One table, not a scatter of switch statements.
var errorTypeByCode = map[errors.Code]string{
	errors.CodeValidation:   "invalid_request_error",
	errors.CodeNotFound:     "invalid_request_error",
	errors.CodeUnauthorized: "authentication_error",
	errors.CodeForbidden:    "permission_error",
	errors.CodeConflict:     "invalid_request_error",
	errors.CodeRateLimited:  "rate_limit_error",
	errors.CodeTimeout:      "api_error",
	errors.CodeUpstream:     "api_error",
	errors.CodeInternal:     "api_error",
}

// writeJSON commits status and encodes v. An encode failure after the
// header is committed means the client disconnected mid-write — there
// is no recovery path, so it is deliberately swallowed after being
// checked (satisfying errcheck honestly: the decision is explicit).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return
	}
}

// writeError serializes any error as the OpenAI-compatible error body.
// Non-AppError values become an opaque 500 — internal details never
// reach the wire (the cause chain is for logs, via the middleware).
func writeError(w http.ResponseWriter, err error) {
	appErr := asAppError(err)

	errType, ok := errorTypeByCode[appErr.Code]
	if !ok {
		errType = "api_error"
	}

	writeJSON(w, appErr.HTTPStatus(), openAIError{Error: openAIErrorBody{
		Message: appErr.Message,
		Type:    errType,
		Code:    string(appErr.Code),
	}})
}

// asAppError normalizes any error to *errors.AppError, mapping unknown
// errors to an opaque internal error rather than exposing their text.
func asAppError(err error) *errors.AppError {
	var appErr *errors.AppError
	if ok := errorsAs(err, &appErr); ok {
		return appErr
	}
	return errors.Wrap(errors.CodeInternal, "internal server error", err)
}
