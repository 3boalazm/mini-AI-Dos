package server

// Database-mode E2E: the full persistent-auth flow against real
// PostgreSQL. Skipped unless TEST_DATABASE_URL is set (the suite never
// depends on Docker). Run with:
//
//	TEST_DATABASE_URL=postgres://mini_ai_dos:mini_ai_dos_local@localhost:5432/mini_ai_dos?sslmode=disable \
//	  go test ./services/gateway/internal/server/ -run TestE2E_DatabaseMode -v
//
// Flow: migrate → generate key → start gateway in database mode →
// health → completion with the generated key (200) → revoke → same
// request again (401) → raw key and hash absent from logs → shutdown.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ai-dos/foundation/logging"
	"github.com/ai-dos/gateway/internal/config"
	"github.com/ai-dos/gateway/internal/provider"
	"github.com/ai-dos/gateway/internal/store"
	"github.com/ai-dos/gateway/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestE2E_DatabaseMode(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping database-mode E2E (see file header for the run command)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1–2. Migrate (idempotent) and prepare a clean key table.
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	if _, err := store.Migrate(ctx, pool, migrations.FS); err != nil {
		pool.Close()
		t.Fatalf("migrating: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM api_keys`); err != nil {
		pool.Close()
		t.Fatalf("cleaning api_keys: %v", err)
	}
	pool.Close()

	repo, err := store.NewPostgres(ctx, url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer repo.Close()

	// 3. Generate a key (what cmd/keygen does).
	raw, key, err := store.GenerateKey("e2e-db", time.Now())
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if err := repo.Create(ctx, key); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 4. Start the gateway in database mode.
	logs := &syncBuffer{}
	cfg := &config.Config{
		Port:     0,
		AuthMode: config.AuthModeDatabase,
		Provider: config.ProviderMock,
		Env:      "production",
		LogLevel: "info",
	}
	log := logging.New(logging.Config{Environment: logging.EnvProduction, Level: "info", Output: logs})
	srv := New(cfg, log, provider.NewMock(), repo)

	ln, err := srv.Listen()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()
	base := fmt.Sprintf("http://%s", ln.Addr().String())
	client := &http.Client{Timeout: 5 * time.Second}

	// 5. Health.
	health, err := doRequest(t, client, http.MethodGet, base+"/health", "", "")
	if err != nil || health.status != http.StatusOK {
		t.Fatalf("health: status %d, err %v", health.status, err)
	}

	chatBody := `{"model":"e2e-db-model","messages":[{"role":"user","content":"db mode ping"}]}`

	// 6–7. Completion with the generated key → 200.
	res, err := doRequest(t, client, http.MethodPost, base+"/v1/chat/completions", raw, chatBody)
	if err != nil {
		t.Fatalf("chat request: %v", err)
	}
	if res.status != http.StatusOK {
		t.Fatalf("chat with db key: got %d — body: %s", res.status, res.body)
	}

	// 8. Revoke.
	if err := repo.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// 9–10. Same request → 401.
	res, err = doRequest(t, client, http.MethodPost, base+"/v1/chat/completions", raw, chatBody)
	if err != nil {
		t.Fatalf("post-revoke request: %v", err)
	}
	if res.status != http.StatusUnauthorized {
		t.Fatalf("revoked key: got %d, want 401", res.status)
	}

	// 11. Logs: metadata present, secrets absent.
	logText := logs.String()
	if !strings.Contains(logText, "revoked API key") {
		t.Error("revocation rejection should be logged")
	}
	if strings.Contains(logText, raw) {
		t.Error("SECURITY: raw API key appeared in logs")
	}
	if strings.Contains(logText, key.KeyHash) {
		t.Error("SECURITY: API key hash appeared in logs")
	}
	if strings.Contains(logText, url) {
		t.Error("SECURITY: DATABASE_URL appeared in logs")
	}

	// 12. Shutdown.
	if err := srv.Shutdown(5 * time.Second); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}
}
