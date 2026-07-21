// Package logging provides the structured logging foundation every
// service imports. It is a thin wrapper over the standard library's
// log/slog, not a replacement for it — the wrapper exists only to
// enforce two things every service must do consistently: emit JSON in
// production, and always be ready to carry a trace_id when tracing
// (turn 6 of the architecture work) is wired in. It contains no
// business logic and no service-specific fields.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Environment selects the output format. Text is for local development
// (human-readable); JSON is for every other environment, since that's
// what the OpenTelemetry collector / Loki pipeline (TDS §10) expects.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

// Config controls how a Logger is built. Every field has a safe
// default via New — nothing here requires a caller to know internals.
type Config struct {
	Environment Environment
	// Level is one of "debug", "info", "warn", "error". Defaults to "info".
	Level string
	// Output defaults to os.Stdout. Overridable for tests.
	Output io.Writer
}

// Logger is the foundation's public type. It embeds *slog.Logger so
// every stdlib slog method (Info, Error, With, and so on) is available
// directly — this is a thin wrapper, not a parallel API to learn.
type Logger struct {
	*slog.Logger
}

// New builds a Logger from Config, applying safe defaults for any
// zero-valued field.
func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	level := parseLevel(cfg.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.Environment == EnvDevelopment {
		handler = slog.NewTextHandler(cfg.Output, opts)
	} else {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	}

	return &Logger{Logger: slog.New(handler)}
}

// contextKey is unexported so no other package can collide with it —
// standard Go idiom for context values.
type contextKey struct{}

var traceIDKey = contextKey{}

// WithTraceID returns a context carrying the given trace ID, so a
// request-scoped logger built later via FromContext can attach it
// automatically. This is the seam tracing (turn 6) plugs into — this
// package does not depend on any tracing library itself.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// FromContext returns a Logger annotated with the request's trace ID
// if one is present in ctx, or l unchanged if not. Every service should
// log through this rather than the raw Logger once inside request
// handling, so every log line is correlatable to its trace.
func (l *Logger) FromContext(ctx context.Context) *Logger {
	traceID, ok := ctx.Value(traceIDKey).(string)
	if !ok || traceID == "" {
		return l
	}
	return &Logger{Logger: l.Logger.With(slog.String("trace_id", traceID))}
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
