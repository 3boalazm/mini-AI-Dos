package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate applies every *.up.sql in fsys that has not been applied
// yet, in lexical (numeric-prefix) order, each inside its own
// transaction. Forward-only by construction: down files are never
// read, and there is no automatic rollback of applied versions.
// Returns the filenames applied this run.
//
// This is deliberately a ~60-line runner, not a migration framework —
// exactly the mechanism specs/database/migrations-strategy.md
// describes (deterministic, forward-only, bookkeeping table), sized to
// what Mini AI-DOS needs. Its bookkeeping table (schema_migrations,
// version = filename) is compatible in spirit but not in format with
// golang-migrate's; pick one runner per database and stay with it.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) ([]string, error) {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, fmt.Errorf("store: creating schema_migrations table: %w", err)
	}

	files, err := fs.Glob(fsys, "*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("store: listing migrations: %w", err)
	}
	sort.Strings(files)

	var applied []string
	for _, name := range files {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
		).Scan(&exists); err != nil {
			return applied, fmt.Errorf("store: checking migration %s: %w", name, err)
		}
		if exists {
			continue
		}

		sql, err := fs.ReadFile(fsys, name)
		if err != nil {
			return applied, fmt.Errorf("store: reading migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("store: beginning transaction for %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("store: migration %s failed (rolled back, nothing partial applied): %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("store: recording migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("store: committing migration %s: %w", name, err)
		}
		applied = append(applied, name)
	}
	return applied, nil
}
