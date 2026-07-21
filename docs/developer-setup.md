# Developer Setup

## Prerequisites

- **Go 1.22+** — the backend. No external Go dependencies exist yet by design (`services/foundation` is stdlib-only); this stays true until epic 2 (Database Layer) genuinely needs a driver.
- **Node 22+** and **pnpm 9+** — the TypeScript side.
- **Docker** and **Docker Compose** — local Postgres, Redis, NATS.

## First-time setup

```bash
git clone <repo-url> ai-dos
cd ai-dos
make install   # pnpm install
make up        # starts Postgres, Redis, NATS via docker-compose.yml
make verify    # confirms your environment matches what CI expects
```

If `make verify` passes, your environment is correctly set up. If it doesn't, that's a real signal — don't work around it locally in a way CI won't also do.

## Day-to-day commands

| Command                 | Does                                                               |
| ----------------------- | ------------------------------------------------------------------ |
| `make build`            | Builds Go foundation + all TypeScript packages                     |
| `make lint`             | `gofmt` + `golangci-lint` (Go) and `eslint` (TypeScript)           |
| `make typecheck`        | `tsc --noEmit` across every TypeScript package                     |
| `make test`             | Go `go test -race` + TypeScript `vitest run`, across every package |
| `make test-coverage`    | Same, with coverage reporting                                      |
| `make fmt`              | Formats everything in place — `gofmt -w` + `prettier --write`      |
| `make up` / `make down` | Start/stop local Postgres, Redis, NATS                             |

## A real Go workspace gotcha, documented so nobody rediscovers it the hard way

`go build ./...` from the repository root does **not** build every module in `go.work` — it fails with `directory prefix . does not contain modules listed in go.work`. This is because the repo root itself isn't a Go module; only `services/foundation` (and future service directories) are. Target each module's own path explicitly: `go build ./services/foundation/...`. The Makefile and CI already do this correctly — if you're running Go commands directly rather than through `make`, use the explicit path form.

## Go dependency policy

`services/foundation` has zero external dependencies, deliberately — everything it needs (structured logging, error handling, UUID generation) exists in the standard library at a quality bar that doesn't justify a dependency. Future service modules (`services/gateway`, `services/registry`, and so on) will have real dependencies (a Postgres driver, for instance) once their epics need them — that's expected and fine. The zero-dependency bar is specific to the foundation layer, not a rule for the whole backend.

## TypeScript package conventions

Every package extends `@ai-dos/tsconfig-base` (strict mode, project references) and `@ai-dos/eslint-config`. `rootDir`/`outDir` are declared per-package, not in the shared base — a shared config's relative paths resolve against _its own_ location, not the extending package's, which is a real TypeScript gotcha this repository's config already works around; see the git history on `packages/tsconfig-base/library.json` if you're curious why it looks the way it does.
