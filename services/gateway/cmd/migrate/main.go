// Command migrate applies the gateway's embedded SQL migrations to the
// database in DATABASE_URL. Deterministic and forward-only: files run
// in numeric order, each exactly once (tracked in schema_migrations),
// each in its own transaction, and nothing is ever rolled back
// automatically.
//
//	DATABASE_URL=postgres://... go run ./services/gateway/cmd/migrate
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	foundationconfig "github.com/ai-dos/foundation/config"
	"github.com/ai-dos/gateway/internal/store"
	"github.com/ai-dos/gateway/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dbTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	required, err := foundationconfig.New().RequireString("DATABASE_URL")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, required["DATABASE_URL"])
	if err != nil {
		return fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("cannot reach PostgreSQL (is it running, and is DATABASE_URL correct?): %w", err)
	}

	applied, err := store.Migrate(ctx, pool, migrations.FS)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		fmt.Println("database is up to date — no migrations to apply")
		return nil
	}
	for _, name := range applied {
		fmt.Printf("applied: %s\n", name)
	}
	return nil
}
