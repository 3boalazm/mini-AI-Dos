// Package config provides typed environment configuration loading.
// It intentionally does not depend on viper or envconfig — the
// foundation layer stays dependency-free by design (see this
// repository's docs/repository-overview.md for why), and the surface
// area this package actually needs (read a string, require it or
// don't, parse an int) doesn't justify an external dependency.
//
// This package loads configuration; it does not define what
// configuration any particular service needs — a service's own config
// struct lives with that service, not here.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Loader reads from a source of key-value pairs — os.Environ by
// default via New, or a map in tests via NewFromMap, so config-reading
// code never has to touch os.Getenv directly and is trivially testable
// without mutating real process environment.
type Loader struct {
	lookup func(key string) (string, bool)
}

// New returns a Loader reading from the real process environment.
func New() *Loader {
	return &Loader{lookup: os.LookupEnv}
}

// NewFromMap returns a Loader reading from an in-memory map — the seam
// every test in this package (and every future service's config tests)
// should use instead of mutating real environment variables.
func NewFromMap(values map[string]string) *Loader {
	return &Loader{
		lookup: func(key string) (string, bool) {
			v, ok := values[key]
			return v, ok
		},
	}
}

// MissingEnvError reports every missing required variable at once,
// not one at a time — failing fast on the first missing variable would
// mean a developer fixes one, reruns, discovers the next, and repeats;
// this reports the complete list on the first run.
type MissingEnvError struct {
	Keys []string
}

func (e *MissingEnvError) Error() string {
	return fmt.Sprintf("missing required environment variables: %v", e.Keys)
}

// RequireString reads each key in keys, collecting every missing one
// into a single MissingEnvError rather than stopping at the first.
func (l *Loader) RequireString(keys ...string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	var missing []string

	for _, key := range keys {
		v, ok := l.lookup(key)
		if !ok || v == "" {
			missing = append(missing, key)
			continue
		}
		result[key] = v
	}

	if len(missing) > 0 {
		return nil, &MissingEnvError{Keys: missing}
	}
	return result, nil
}

// OptionalString returns the value for key, or fallback if unset.
func (l *Loader) OptionalString(key, fallback string) string {
	if v, ok := l.lookup(key); ok && v != "" {
		return v
	}
	return fallback
}

// OptionalInt returns the parsed integer value for key, or fallback if
// unset or unparseable. An unparseable value is treated the same as
// unset rather than surfaced as an error — this is an optional
// setting; a malformed optional setting falling back to a safe default
// is preferable to a service failing to start over it. Required,
// type-checked settings should use RequireString and parse explicitly
// at the call site, where a parse failure can be treated as the
// startup error it actually is.
func (l *Loader) OptionalInt(key string, fallback int) int {
	v, ok := l.lookup(key)
	if !ok || v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
