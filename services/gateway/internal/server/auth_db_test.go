package server

// Database-auth-mode tests against a fake repository — the auth
// decision logic (hash → lookup → revocation → identical external
// 401s) verified without PostgreSQL. The Postgres implementation
// itself is covered by the gated integration tests in
// internal/store/postgres_test.go.

import (
	"context"
	stderrors "errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/config"
	"github.com/ai-dos/gateway/internal/provider"
	"github.com/ai-dos/gateway/internal/store"
)

// fakeRepo is an in-memory store.Repository for auth tests.
type fakeRepo struct {
	byHash map[string]*store.APIKey
	err    error // returned by every call when set — simulates DB failure
}

func (f *fakeRepo) FindByHash(ctx context.Context, hash string) (*store.APIKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	k, ok := f.byHash[hash]
	if !ok {
		return nil, store.ErrNotFound
	}
	return k, nil
}

func (f *fakeRepo) Create(ctx context.Context, key *store.APIKey) error {
	if f.err != nil {
		return f.err
	}
	if _, exists := f.byHash[key.KeyHash]; exists {
		return store.ErrDuplicate
	}
	f.byHash[key.KeyHash] = key
	return nil
}

func (f *fakeRepo) Revoke(ctx context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	for _, k := range f.byHash {
		if k.ID == id && k.RevokedAt == nil {
			now := time.Now()
			k.RevokedAt = &now
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeRepo) List(ctx context.Context) ([]store.APIKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	var keys []store.APIKey
	for _, k := range f.byHash {
		keys = append(keys, *k)
	}
	return keys, nil
}

// newDBTestServer wires a database-auth-mode server around a fake
// repository, returning the handler, the repo, and one valid raw key.
func newDBTestServer(t *testing.T) (http.Handler, *fakeRepo, string) {
	t.Helper()
	raw, key, err := store.GenerateKey("test", time.Now())
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	repo := &fakeRepo{byHash: map[string]*store.APIKey{key.KeyHash: key}}

	cfg := &config.Config{
		Port:     8080,
		AuthMode: config.AuthModeDatabase,
		Provider: config.ProviderMock,
		Env:      "development",
		LogLevel: "error",
	}
	log := logging.New(logging.Config{Environment: logging.EnvDevelopment, Output: io.Discard})
	return New(cfg, log, provider.NewMock(), repo).httpSrv.Handler, repo, raw
}

func TestDBAuth_ValidKey200(t *testing.T) {
	h, _, raw := newDBTestServer(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", raw, validChat)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid db key: got %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
}

func TestDBAuth_UnknownKey401(t *testing.T) {
	h, _, _ := newDBTestServer(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", "mad_"+strings.Repeat("0", 48), validChat)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown key: got %d, want 401", rec.Code)
	}
}

func TestDBAuth_RevokedKey401_SameMessageAsUnknown(t *testing.T) {
	h, repo, raw := newDBTestServer(t)

	unknownRec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", "mad_"+strings.Repeat("0", 48), validChat)

	key := repo.byHash[store.HashKey(raw)]
	if err := repo.Revoke(context.Background(), key.ID); err != nil {
		t.Fatalf("revoking: %v", err)
	}
	revokedRec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", raw, validChat)

	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key: got %d, want 401", revokedRec.Code)
	}
	// A caller must not be able to distinguish "unknown" from "revoked".
	if unknownRec.Body.String() != revokedRec.Body.String() {
		t.Errorf("revoked and unknown keys must produce identical bodies:\nunknown: %s\nrevoked: %s",
			unknownRec.Body.String(), revokedRec.Body.String())
	}
}

func TestDBAuth_DatabaseFailure500Not401(t *testing.T) {
	h, repo, raw := newDBTestServer(t)
	repo.err = stderrors.New("connection refused to 10.0.0.99:5432")

	rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", raw, validChat)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("db failure: got %d, want 500 (an outage is not an auth verdict)", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.99") {
		t.Error("internal database error detail leaked to the caller")
	}
}

func TestDBAuth_EnvKeyDoesNotWorkInDatabaseMode(t *testing.T) {
	h, _, _ := newDBTestServer(t)
	// Even if an env key were configured, database mode must not
	// consult it — no silent fallback in either direction.
	rec := doJSON(t, h, http.MethodPost, "/v1/chat/completions", "test-gateway-key", validChat)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("env-style key in db mode: got %d, want 401", rec.Code)
	}
}

func TestNew_RejectsMismatchedRepoWiring(t *testing.T) {
	log := logging.New(logging.Config{Environment: logging.EnvDevelopment, Output: io.Discard})

	assertPanics := func(name string, fn func()) {
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}

	assertPanics("database mode without repo", func() {
		New(&config.Config{AuthMode: config.AuthModeDatabase, Provider: config.ProviderMock},
			log, provider.NewMock(), nil)
	})
	assertPanics("env mode with repo", func() {
		New(&config.Config{AuthMode: config.AuthModeEnv, APIKey: "k", Provider: config.ProviderMock},
			log, provider.NewMock(), &fakeRepo{byHash: map[string]*store.APIKey{}})
	})
}
