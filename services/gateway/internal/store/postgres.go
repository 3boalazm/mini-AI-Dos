package store

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation is the Postgres error code for a unique constraint
// violation — how a duplicate key_hash insert surfaces.
const pgUniqueViolation = "23505"

// Postgres implements Repository over a pgx connection pool. The pool
// handles connection reuse; this type holds no other state.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres connects and verifies the database is actually reachable
// (fail fast at startup, not on the first request). The URL is never
// logged by this package — it contains credentials.
func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: invalid DATABASE_URL: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: cannot reach PostgreSQL (is it running, and is DATABASE_URL correct?): %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close releases the pool. Safe to call once during shutdown.
func (p *Postgres) Close() { p.pool.Close() }

// Ready verifies the schema this repository needs actually exists —
// the difference between "Postgres is up" and "migrations have run".
func (p *Postgres) Ready(ctx context.Context) error {
	var regclass *string
	if err := p.pool.QueryRow(ctx, `SELECT to_regclass('api_keys')::text`).Scan(&regclass); err != nil {
		return fmt.Errorf("store: readiness check failed: %w", err)
	}
	if regclass == nil {
		return stderrors.New("store: api_keys table does not exist — run migrations first (go run ./services/gateway/cmd/migrate)")
	}
	return nil
}

func (p *Postgres) FindByHash(ctx context.Context, hash string) (*APIKey, error) {
	var k APIKey
	err := p.pool.QueryRow(ctx,
		`SELECT id, key_hash, key_prefix, name, created_at, revoked_at
		   FROM api_keys WHERE key_hash = $1`, hash,
	).Scan(&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Name, &k.CreatedAt, &k.RevokedAt)
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: find by hash: %w", err)
	}
	return &k, nil
}

func (p *Postgres) Create(ctx context.Context, key *APIKey) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		key.ID, key.KeyHash, key.KeyPrefix, key.Name, key.CreatedAt,
	)
	var pgErr *pgconn.PgError
	if stderrors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrDuplicate
	}
	if err != nil {
		return fmt.Errorf("store: create api key: %w", err)
	}
	return nil
}

func (p *Postgres) Revoke(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("store: revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) List(ctx context.Context) ([]APIKey, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, key_hash, key_prefix, name, created_at, revoked_at
		   FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Name, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("store: scan api key row: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate api key rows: %w", err)
	}
	return keys, nil
}
