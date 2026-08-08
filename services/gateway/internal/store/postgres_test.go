package store

// Integration tests against real PostgreSQL. They are skipped unless
// TEST_DATABASE_URL is set, so the default test suite never depends on
// Docker. Run them with:
//
//	TEST_DATABASE_URL=postgres://mini_ai_dos:mini_ai_dos_local@localhost:5432/mini_ai_dos?sslmode=disable \
//	  go test ./services/gateway/internal/store/ -run TestPostgres -v
//
// Each run migrates via the embedded migrations and cleans the
// api_keys table before starting, so runs are repeatable.

import (
	"context"
	stderrors "errors"
	"os"
	"testing"
	"time"

	"github.com/ai-dos/gateway/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func postgresRepo(t *testing.T) *Postgres {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping PostgreSQL integration tests (see file header for the run command)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	if _, err := Migrate(ctx, pool, migrations.FS); err != nil {
		pool.Close()
		t.Fatalf("migrating: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM api_keys`); err != nil {
		pool.Close()
		t.Fatalf("cleaning api_keys: %v", err)
	}
	pool.Close()

	repo, err := NewPostgres(ctx, url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	t.Cleanup(repo.Close)
	if err := repo.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	return repo
}

func mustGenerate(t *testing.T, name string) (string, *APIKey) {
	t.Helper()
	raw, key, err := GenerateKey(name, time.Now())
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return raw, key
}

func TestPostgres_CreateAndFindActiveKey(t *testing.T) {
	repo := postgresRepo(t)
	ctx := context.Background()

	raw, key := mustGenerate(t, "integration")
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByHash(ctx, HashKey(raw))
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found.ID != key.ID || found.KeyPrefix != key.KeyPrefix || found.Name != "integration" {
		t.Errorf("round-trip mismatch: %+v vs %+v", found, key)
	}
	if found.Revoked() {
		t.Error("fresh key must be active")
	}
}

func TestPostgres_UnknownKey(t *testing.T) {
	repo := postgresRepo(t)
	_, err := repo.FindByHash(context.Background(), HashKey("mad_does_not_exist"))
	if !stderrors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestPostgres_RevokedKey(t *testing.T) {
	repo := postgresRepo(t)
	ctx := context.Background()

	raw, key := mustGenerate(t, "to-revoke")
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	found, err := repo.FindByHash(ctx, HashKey(raw))
	if err != nil {
		t.Fatalf("FindByHash after revoke: %v", err)
	}
	if !found.Revoked() {
		t.Error("key should report revoked")
	}

	// Re-revoking is ErrNotFound (no active key with that id).
	if err := repo.Revoke(ctx, key.ID); !stderrors.Is(err, ErrNotFound) {
		t.Errorf("re-revoke: got %v, want ErrNotFound", err)
	}
}

func TestPostgres_DuplicateHashRejected(t *testing.T) {
	repo := postgresRepo(t)
	ctx := context.Background()

	_, key := mustGenerate(t, "dup")
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	clone := *key
	clone.ID = key.ID + "x" // different id, same hash
	if err := repo.Create(ctx, &clone); !stderrors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate hash: got %v, want ErrDuplicate", err)
	}
}

func TestPostgres_ContextCancellation(t *testing.T) {
	repo := postgresRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repo.FindByHash(ctx, HashKey("mad_x")); err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestPostgres_MigrateIsIdempotent(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()

	if _, err := Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	applied, err := Migrate(ctx, pool, migrations.FS)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("second run should apply nothing, applied %v", applied)
	}
}

func TestPostgres_List(t *testing.T) {
	repo := postgresRepo(t)
	ctx := context.Background()

	_, k1 := mustGenerate(t, "first")
	_, k2 := mustGenerate(t, "second")
	for _, k := range []*APIKey{k1, k2} {
		if err := repo.Create(ctx, k); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	keys, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("List: got %d keys, want 2", len(keys))
	}
}
