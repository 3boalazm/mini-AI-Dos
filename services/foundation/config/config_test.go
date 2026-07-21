package config

import (
	"errors"
	"testing"
)

func TestRequireString_AllPresent(t *testing.T) {
	l := NewFromMap(map[string]string{"DATABASE_URL": "postgres://x", "PORT": "8080"})

	values, err := l.RequireString("DATABASE_URL", "PORT")
	if err != nil {
		t.Fatalf("expected no error when all keys present, got: %v", err)
	}
	if values["DATABASE_URL"] != "postgres://x" {
		t.Errorf("expected DATABASE_URL to be returned, got %v", values)
	}
}

func TestRequireString_CollectsAllMissing_NotJustFirst(t *testing.T) {
	l := NewFromMap(map[string]string{"PORT": "8080"})

	_, err := l.RequireString("DATABASE_URL", "REDIS_URL", "PORT", "NATS_URL")
	if err == nil {
		t.Fatal("expected an error when required keys are missing")
	}

	var missingErr *MissingEnvError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected *MissingEnvError, got %T", err)
	}
	if len(missingErr.Keys) != 3 {
		t.Errorf("expected all 3 missing keys reported at once, got %d: %v", len(missingErr.Keys), missingErr.Keys)
	}
}

func TestRequireString_EmptyStringTreatedAsMissing(t *testing.T) {
	l := NewFromMap(map[string]string{"DATABASE_URL": ""})

	_, err := l.RequireString("DATABASE_URL")
	if err == nil {
		t.Error("expected an empty-string value to be treated as missing, not as a valid empty config")
	}
}

func TestOptionalString_FallsBackWhenUnset(t *testing.T) {
	l := NewFromMap(map[string]string{})
	if got := l.OptionalString("LOG_LEVEL", "info"); got != "info" {
		t.Errorf("expected fallback 'info', got %q", got)
	}
}

func TestOptionalInt_FallsBackOnUnparseable(t *testing.T) {
	l := NewFromMap(map[string]string{"MAX_CONNECTIONS": "not-a-number"})
	if got := l.OptionalInt("MAX_CONNECTIONS", 10); got != 10 {
		t.Errorf("expected unparseable optional value to fall back to 10, got %d", got)
	}
}

func TestOptionalInt_ParsesValidValue(t *testing.T) {
	l := NewFromMap(map[string]string{"MAX_CONNECTIONS": "42"})
	if got := l.OptionalInt("MAX_CONNECTIONS", 10); got != 42 {
		t.Errorf("expected parsed value 42, got %d", got)
	}
}
