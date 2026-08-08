# Mini AI-DOS

A small AI gateway: one OpenAI-compatible endpoint, one API key, a
pluggable provider behind it. Go for the gateway (`services/`),
TypeScript workspace for shared tooling and the future SDK
(`packages/`, `sdk/`).

Mini AI-DOS is deliberately not the full AI-DOS platform. No
organizations, no RBAC, no billing, no benchmarking, no plugins, no
event bus — a caller sends a chat completion request with a Bearer
key, the gateway validates it, forwards it to the configured provider,
and returns the normalized OpenAI-shaped response.

## Quick start

```bash
git clone <repo-url> mini-ai-dos
cd mini-ai-dos
cp .env.example .env        # then edit: set MINI_AI_DOS_API_KEY
```

Run it directly (no Docker, no database, no upstream account needed —
the default `mock` provider echoes requests):

```bash
MINI_AI_DOS_API_KEY=dev-key go run ./services/gateway/cmd/gateway
```

Check it:

```bash
curl http://localhost:8080/health
```

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer dev-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"any-model","messages":[{"role":"user","content":"hello"}]}'
```

To talk to a real OpenAI-compatible provider, set in `.env` (or the
environment): `AI_PROVIDER=openai`, `AI_API_KEY=<your upstream key>`,
and optionally `AI_BASE_URL` / `AI_MODEL`. Every variable is
documented in [.env.example](.env.example) — nothing else is read.

### Docker

```bash
docker compose up -d --build
```

brings up the gateway (port 8080) and Postgres. Requires
`MINI_AI_DOS_API_KEY` in `.env`.

### Database and migrations

The gateway runtime does not use a database yet — authentication is
the single `MINI_AI_DOS_API_KEY`. A reviewed, hash-only-storage schema
for persistent API keys exists at
[services/gateway/migrations/](services/gateway/migrations/); apply it
with [golang-migrate](https://github.com/golang-migrate/migrate) once
Postgres is running:

```bash
migrate -path services/gateway/migrations -database "postgres://mini_ai_dos:mini_ai_dos_local@localhost:5432/mini_ai_dos?sslmode=disable" up
```

## API

Full contract: [services/gateway/openapi.yaml](services/gateway/openapi.yaml).

| Route                       | Auth       | Notes                            |
| --------------------------- | ---------- | -------------------------------- |
| `GET /health`               | none       | liveness + configured provider   |
| `POST /v1/chat/completions` | Bearer key | OpenAI-compatible, non-streaming |

Errors use the OpenAI error body (`{"error":{"message","type","code"}}`)
with predictable statuses: 400 invalid request, 401 bad/missing key,
413 body over 1 MiB, 415 wrong Content-Type, 429 rate limit (gateway
or upstream), 500 internal, 502 upstream failure, 504 upstream
timeout.

## Development

```bash
make install   # TypeScript dependencies (pnpm)
make test      # Go + TypeScript test suites
make verify    # install + lint + typecheck + build + test — the full CI sequence
make run       # run the gateway locally
```

Layout:

```
services/foundation/   config, errors, logging, util — Go, stdlib-only, shared by services
services/gateway/      the Mini AI-DOS gateway (cmd/gateway + internal/*)
packages/              shared TypeScript tooling (tsconfig, eslint, shared-types)
sdk/typescript/        SDK error types (no client methods yet)
specs/                 the parent AI-DOS contract reference (read-only; the gateway's
                       own implemented contract is services/gateway/openapi.yaml)
docs/                  developer setup, contributing, architecture notes
```

## Known limitations

- **No streaming** (`stream: true` is rejected with 400 rather than silently ignored).
- **Single API key** from the environment; the persistent multi-key model exists only as a migration, not wired to the runtime (database work is blocked on Docker/WSL2 on the primary dev machine).
- **Message content is plain strings** — multi-part content arrays are rejected.
- **Rate limiting is in-process** (fixed window, single instance). Running multiple replicas multiplies the effective limit.
- `-race` is unavailable on the primary dev machine (no C compiler); CI runs it.
