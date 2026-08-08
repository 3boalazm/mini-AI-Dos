// Command keygen manages Mini AI-DOS database API keys: create (the
// default), -revoke, and -list. It is deliberately a CLI, not an admin
// API — key management is an operator action at the machine, not a
// network surface.
//
//	DATABASE_URL=postgres://... go run ./services/gateway/cmd/keygen -name "my key"
//	DATABASE_URL=postgres://... go run ./services/gateway/cmd/keygen -list
//	DATABASE_URL=postgres://... go run ./services/gateway/cmd/keygen -revoke <key-id>
//
// The raw key is printed exactly once, to stdout, at creation. It is
// never written to disk and never stored — only its SHA-256 hash
// reaches PostgreSQL. Key hashes are never printed by any subcommand.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	foundationconfig "github.com/ai-dos/foundation/config"
	"github.com/ai-dos/gateway/internal/store"
)

const dbTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "keygen: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	name := flag.String("name", "", "optional label for the new key")
	revoke := flag.String("revoke", "", "revoke the key with this id instead of creating one")
	list := flag.Bool("list", false, "list keys (id, prefix, name, status) instead of creating one")
	flag.Parse()

	required, err := foundationconfig.New().RequireString("DATABASE_URL")
	if err != nil {
		return fmt.Errorf("%w (keygen operates on the PostgreSQL key store only)", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	repo, err := store.NewPostgres(ctx, required["DATABASE_URL"])
	if err != nil {
		return err
	}
	defer repo.Close()
	if err := repo.Ready(ctx); err != nil {
		return err
	}

	switch {
	case *list:
		return listKeys(ctx, repo)
	case *revoke != "":
		return revokeKey(ctx, repo, *revoke)
	default:
		return createKey(ctx, repo, *name)
	}
}

func createKey(ctx context.Context, repo store.Repository, name string) error {
	raw, key, err := store.GenerateKey(name, time.Now())
	if err != nil {
		return err
	}
	if err := repo.Create(ctx, key); err != nil {
		return err
	}

	fmt.Println("API KEY CREATED")
	fmt.Println()
	fmt.Printf("ID:     %s\n", key.ID)
	fmt.Printf("Name:   %s\n", displayName(key.Name))
	fmt.Printf("Prefix: %s\n", key.KeyPrefix)
	fmt.Printf("Key:    %s\n", raw)
	fmt.Println()
	fmt.Println("This is the ONLY time the key is shown. Store it securely now;")
	fmt.Println("only its hash is in the database and it cannot be recovered.")
	return nil
}

func revokeKey(ctx context.Context, repo store.Repository, id string) error {
	if err := repo.Revoke(ctx, id); err != nil {
		return fmt.Errorf("revoking %s: %w (use -list to see key ids; already-revoked keys cannot be re-revoked)", id, err)
	}
	fmt.Printf("API KEY REVOKED: %s\n", id)
	return nil
}

func listKeys(ctx context.Context, repo store.Repository) error {
	keys, err := repo.List(ctx)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		fmt.Println("no api keys exist yet — create one with: keygen -name \"my key\"")
		return nil
	}
	// Hashes are deliberately never printed.
	fmt.Printf("%-38s %-14s %-22s %-22s %s\n", "ID", "PREFIX", "NAME", "CREATED", "STATUS")
	for _, k := range keys {
		status := "active"
		if k.Revoked() {
			status = "revoked " + k.RevokedAt.UTC().Format(time.RFC3339)
		}
		fmt.Printf("%-38s %-14s %-22s %-22s %s\n",
			k.ID, k.KeyPrefix, displayName(k.Name), k.CreatedAt.UTC().Format(time.RFC3339), status)
	}
	return nil
}

func displayName(name string) string {
	if name == "" {
		return "(unnamed)"
	}
	return name
}
