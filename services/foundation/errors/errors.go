// Package errors provides the base error foundation every service
// builds on: a common error shape, a code registry, and serialization
// to the ProblemDetails format contracts/validation-rules.md specifies
// for internal-facing routes.
//
// This package is deliberately generic. It does NOT define
// ProviderError or its nine subclasses (RateLimitedError, AuthError,
// and so on) — those are the Provider Adapter Framework's contract
// (epic 5 in the roadmap), not Project Foundation's. Defining them
// here would mean epic 1 reaching into epic 5's scope before epic 5
// exists to own it. What this package defines instead is generic
// enough that epic 5 imports and extends it, rather than duplicating
// it.
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Code is a stable, wire-visible error identifier — never a raw string
// scattered across call sites, so a typo can't silently produce an
// error nobody's error-mapping table recognizes.
type Code string

const (
	CodeNotFound     Code = "not_found"
	CodeValidation   Code = "validation_error"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeConflict     Code = "conflict"
	CodeInternal     Code = "internal_error"
)

// httpStatusByCode is the single source of truth for code-to-status
// mapping — a switch statement scattered across handlers would let two
// call sites disagree about what CodeNotFound means; a map can't.
var httpStatusByCode = map[Code]int{
	CodeNotFound:     http.StatusNotFound,
	CodeValidation:   http.StatusBadRequest,
	CodeUnauthorized: http.StatusUnauthorized,
	CodeForbidden:    http.StatusForbidden,
	CodeConflict:     http.StatusConflict,
	CodeInternal:     http.StatusInternalServerError,
}

// AppError is the base error type every service-level error should
// wrap or embed. It satisfies the standard error interface, so it
// composes with errors.Is/errors.As/fmt.Errorf(%w) exactly like any
// other Go error — nothing about this requires special-casing at call
// sites that only care "did this fail."
type AppError struct {
	Code    Code
	Message string
	// Cause is the underlying error, if any — preserved for logging and
	// errors.Unwrap, never serialized to a caller (see ProblemDetails).
	Cause error
}

func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code Code, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, Cause: cause}
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// HTTPStatus returns the status code this error should produce at an
// API boundary. Unrecognized codes map to 500 — a new Code added
// without a corresponding httpStatusByCode entry fails safe (opaque
// server error) rather than defaulting to 200 or panicking.
func (e *AppError) HTTPStatus() int {
	if status, ok := httpStatusByCode[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// ProblemDetails is the RFC 7807 shape contracts/validation-rules.md
// specifies for internal-facing routes. Type is left for a caller to
// populate with a real URI if one exists; it is not fabricated here.
type ProblemDetails struct {
	Type   string `json:"type,omitempty"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// ToProblemDetails serializes e into the shape an HTTP handler writes
// to the response body. Cause is intentionally never included — an
// internal error's underlying cause belongs in a log line (via the
// logging package), not in a response a caller outside this service
// can read.
func (e *AppError) ToProblemDetails() ProblemDetails {
	return ProblemDetails{
		Title:  string(e.Code),
		Status: e.HTTPStatus(),
		Detail: e.Message,
	}
}

// MarshalJSON lets an *AppError be json.Marshal'd directly as its
// ProblemDetails form, so a handler can write(err) without an
// intermediate conversion step at every call site.
func (e *AppError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.ToProblemDetails())
}
