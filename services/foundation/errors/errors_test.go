package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPStatus_KnownCodes(t *testing.T) {
	cases := map[Code]int{
		CodeNotFound:     http.StatusNotFound,
		CodeValidation:   http.StatusBadRequest,
		CodeUnauthorized: http.StatusUnauthorized,
		CodeForbidden:    http.StatusForbidden,
		CodeConflict:     http.StatusConflict,
		CodeInternal:     http.StatusInternalServerError,
	}
	for code, want := range cases {
		got := New(code, "x").HTTPStatus()
		if got != want {
			t.Errorf("Code %s: got status %d, want %d", code, got, want)
		}
	}
}

func TestHTTPStatus_UnknownCodeFailsSafe(t *testing.T) {
	err := New(Code("something_nobody_registered"), "x")
	if got := err.HTTPStatus(); got != http.StatusInternalServerError {
		t.Errorf("unregistered code: got %d, want %d (fail-safe default)", got, http.StatusInternalServerError)
	}
}

func TestToProblemDetails_NeverLeaksCause(t *testing.T) {
	underlying := fmt.Errorf("postgres: connection refused on internal host 10.0.0.5")
	wrapped := Wrap(CodeInternal, "could not complete request", underlying)

	pd := wrapped.ToProblemDetails()
	body, err := json.Marshal(pd)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	if pd.Detail != "could not complete request" {
		t.Errorf("expected Detail to be the safe message, got %q", pd.Detail)
	}
	// The real assertion: the internal cause must never reach the wire.
	if strings.Contains(string(body), "10.0.0.5") || strings.Contains(string(body), "postgres") {
		t.Errorf("internal cause leaked into serialized ProblemDetails: %s", body)
	}
}

func TestMarshalJSON_DirectlyMarshalable(t *testing.T) {
	err := New(CodeNotFound, "provider not found")
	body, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("expected *AppError to marshal directly, got error: %v", marshalErr)
	}

	var pd ProblemDetails
	if unmarshalErr := json.Unmarshal(body, &pd); unmarshalErr != nil {
		t.Fatalf("expected valid ProblemDetails JSON, got error: %v", unmarshalErr)
	}
	if pd.Status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", pd.Status)
	}
}

func TestComposesWithStandardErrorsPackage(t *testing.T) {
	underlying := fmt.Errorf("sentinel failure")
	wrapped := Wrap(CodeInternal, "wrapped", underlying)

	// errors.Is / errors.Unwrap must work exactly as they would for any
	// other Go error — AppError is not a special case a caller has to
	// know about.
	if !errors.Is(wrapped, underlying) {
		t.Error("expected errors.Is to find the wrapped underlying error")
	}

	var target *AppError
	if !errors.As(fmt.Errorf("outer: %w", wrapped), &target) {
		t.Error("expected errors.As to unwrap to *AppError through an additional layer of wrapping")
	}
	if target.Code != CodeInternal {
		t.Errorf("expected unwrapped error to preserve Code, got %s", target.Code)
	}
}
