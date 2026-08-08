package server

import (
	"context"
	"crypto/subtle"
	stderrors "errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ai-dos/foundation/errors"
	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/foundation/util"
	"github.com/ai-dos/gateway/internal/store"
)

// errorsAs is stdlib errors.As, aliased because this package imports
// the foundation errors package under the standard name.
func errorsAs(err error, target any) bool { return stderrors.As(err, target) }

// requestMeta is filled in by handlers (provider, model) and read by
// the logging middleware after the handler returns, so the access log
// line can carry completion context without handlers doing their own
// access logging.
type requestMeta struct {
	Provider string
	Model    string
}

type requestMetaKey struct{}

func metaFromContext(ctx context.Context) *requestMeta {
	m, _ := ctx.Value(requestMetaKey{}).(*requestMeta)
	return m
}

// statusRecorder captures the response status for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// withObservability assigns every request a request ID (UUIDv7, doubles
// as the trace ID the foundation logger picks up), recovers panics into
// clean 500s, and writes one structured access-log line per request.
// It never logs headers, bodies, or query strings — request metadata
// only, per the observability rules.
func (s *Server) withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, err := util.NewUUIDv7()
		if err != nil {
			// Random source failure is severe but must not take the
			// request down — fall back to an unidentified request.
			reqID = "unknown"
		}

		ctx := logging.WithTraceID(r.Context(), reqID)
		meta := &requestMeta{}
		ctx = context.WithValue(ctx, requestMetaKey{}, meta)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-Id", reqID)
		rec := &statusRecorder{ResponseWriter: w}
		start := time.Now()

		defer func() {
			if p := recover(); p != nil {
				s.log.FromContext(ctx).Error("panic recovered", "panic", p, "path", r.URL.Path)
				if rec.status == 0 {
					writeError(rec, errors.New(errors.CodeInternal, "internal server error"))
				}
			}

			attrs := []any{
				"request_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if meta.Provider != "" {
				attrs = append(attrs, "provider", meta.Provider)
			}
			if meta.Model != "" {
				attrs = append(attrs, "model", meta.Model)
			}
			s.log.FromContext(ctx).Info("request", attrs...)
		}()

		next.ServeHTTP(rec, r)
	})
}

// withAuth enforces API-key authentication as a Bearer token. The
// presented value is never logged in any path.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			writeError(w, errors.New(errors.CodeUnauthorized, "missing API key: set the Authorization: Bearer header"))
			return
		}
		if err := s.authenticate(r.Context(), token); err != nil {
			writeError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// errInvalidKey is the single external authentication failure. Unknown
// key, revoked key, and wrong env key all produce exactly this — a
// caller learns nothing about WHY a key was rejected.
func errInvalidKey() error {
	return errors.New(errors.CodeUnauthorized, "invalid API key")
}

// authenticate verifies token against the configured auth mode. Env
// mode: constant-time comparison against MINI_AI_DOS_API_KEY. Database
// mode: SHA-256 hash → repository lookup → revocation check. A
// database failure is an internal error (500), never a 401 and never a
// fallback to the env key.
func (s *Server) authenticate(ctx context.Context, token string) error {
	if s.repo == nil {
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.APIKey)) != 1 {
			s.log.FromContext(ctx).Warn("rejected request with invalid API key")
			return errInvalidKey()
		}
		return nil
	}

	key, err := s.repo.FindByHash(ctx, store.HashKey(token))
	if stderrors.Is(err, store.ErrNotFound) {
		s.log.FromContext(ctx).Warn("rejected request with unknown API key")
		return errInvalidKey()
	}
	if err != nil {
		// The real error goes to the log; the caller gets an opaque 500.
		s.log.FromContext(ctx).Error("api key lookup failed", "error", err.Error())
		return errors.Wrap(errors.CodeInternal, "authentication is temporarily unavailable", err)
	}
	if key.Revoked() {
		// key_prefix is identification, not secret material — safe to log.
		s.log.FromContext(ctx).Warn("rejected request with revoked API key", "key_prefix", key.KeyPrefix)
		return errInvalidKey()
	}
	return nil
}

// withRateLimit applies the in-process limiter when enabled.
func (s *Server) withRateLimit(next http.Handler) http.Handler {
	if s.limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := s.limiter.Allow()
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, errors.New(errors.CodeRateLimited, "rate limit exceeded, retry later"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
