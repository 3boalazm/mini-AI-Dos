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

Authentication has two explicit modes — `API_KEY_AUTH_MODE=env` (one
key from the environment, zero database) and
`API_KEY_AUTH_MODE=database` (hashed keys in PostgreSQL, revocable).
There is no fallback between them: a misconfigured database mode fails
at startup instead of silently degrading.

## Development mode (no database)

```bash
git clone <repo-url> mini-ai-dos
cd mini-ai-dos
```

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

The default `mock` provider echoes requests, so this works with no
upstream account. To talk to a real OpenAI-compatible provider, set
`AI_PROVIDER=openai`, `AI_API_KEY=<your upstream key>`, and optionally
`AI_BASE_URL` / `AI_MODEL`. Every variable is documented in
[.env.example](.env.example) — nothing else is read.

### Local models with Ollama (free, on-device)

[Ollama](https://ollama.com) exposes an OpenAI-compatible API, so the
`openai` provider talks to it unchanged — point `AI_BASE_URL` at it.
Two PowerShell helpers in [scripts/](scripts/) wrap the setup:

```powershell
ollama pull qwen2.5-coder:7b
.\scripts\dev-local.ps1                      # gateway on :8080 → qwen2.5-coder:7b
.\scripts\dev-local.ps1 -Model llama3.1:8b   # any pulled model; -Model mock needs no Ollama
```

```powershell
.\scripts\ask.ps1 "Write a Go table-driven test for clampLimit"
.\scripts\ask.ps1 "لخّص الملف ده" -File .\README.md -System "أجب بالعربية"
```

`ask.ps1` sends UTF-8 and prints only the reply, so non-ASCII prompts
work from the Windows console. Any OpenAI-compatible tool (IDE
assistants, SDKs) can use the gateway the same way: base URL
`http://localhost:8080/v1`, API key = `MINI_AI_DOS_API_KEY`.

## Persistent mode (PostgreSQL-backed API keys)

Start PostgreSQL and set up the environment (the URL below matches
docker-compose's development credentials):

```bash
docker compose up -d postgres
```

```bash
export DATABASE_URL="postgres://mini_ai_dos:mini_ai_dos_local@localhost:5432/mini_ai_dos?sslmode=disable"
```

Apply migrations (forward-only, each runs exactly once):

```bash
go run ./services/gateway/cmd/migrate
```

Create an API key — the raw key is printed exactly once and only its
SHA-256 hash is stored:

```bash
go run ./services/gateway/cmd/keygen -name "my first key"
```

Start the gateway in database mode (fails fast if PostgreSQL is
unreachable or unmigrated):

```bash
API_KEY_AUTH_MODE=database go run ./services/gateway/cmd/gateway
```

Use it (substitute the key `keygen` printed):

```bash
curl http://localhost:8080/health
```

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mad_YOUR_GENERATED_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"any-model","messages":[{"role":"user","content":"hello"}]}'
```

Revoke a key (find its id with `-list`); the next request with it
returns 401:

```bash
go run ./services/gateway/cmd/keygen -list
```

```bash
go run ./services/gateway/cmd/keygen -revoke <key-id>
```

### Docker (full stack)

```bash
docker compose up -d --build
```

brings up the gateway (port 8080) and Postgres — nothing else. Set
`MINI_AI_DOS_API_KEY` (env mode, the compose default) or
`API_KEY_AUTH_MODE=database` in `.env`.

### Database-backed tests

The default test suite never needs a database. With PostgreSQL up, the
integration and E2E layers run too:

```bash
TEST_DATABASE_URL="postgres://mini_ai_dos:mini_ai_dos_local@localhost:5432/mini_ai_dos?sslmode=disable" go test ./services/gateway/... -v -run 'Postgres|DatabaseMode'
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
- **Message content is plain strings** — multi-part content arrays are rejected.
- **Rate limiting is in-process** (fixed window, single instance). Running multiple replicas multiplies the effective limit.
- **Key management is a CLI, not an API** — `keygen` is deliberately an operator tool; there is no admin HTTP surface.
- Database-dependent tests require a reachable PostgreSQL (`TEST_DATABASE_URL`); on the primary dev machine Docker needs WSL2, so they run in CI/elsewhere.
- `-race` is unavailable on the primary dev machine (no C compiler); CI runs it.
