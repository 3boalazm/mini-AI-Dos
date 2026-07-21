# AI-DOS Provider Gateway

A polyglot monorepo: Go for the request-serving backend (`services/`), TypeScript for the client SDK and dashboard (`sdk/`, `apps/`). See `docs/repository-overview.md` for why it's split this way, not assumed.

This repository is Project Foundation only — no database, no auth, no providers, no gateway, no business logic. It exists so every subsequent piece of work (roadmap epics 2 through 20) has real tooling, real conventions, and a real CI pipeline to build against from day one, instead of each epic inventing its own.

## Quick start

```bash
make install   # TypeScript dependencies
make up        # local Postgres, Redis, NATS
make verify    # install + lint + typecheck + build + test — the full CI sequence, locally
```

Full setup detail, including what `go` version and why: `docs/developer-setup.md`.

## Structure

```
apps/          TypeScript apps (dashboard — foundation scaffold only, epic 15 builds it out)
packages/      Shared TypeScript packages (tsconfig, eslint config, shared-types)
services/      Go backend (foundation/ today; gateway/, registry/, etc. join as their epics start)
sdk/           Client SDKs (typescript/ today — error types only, no API-calling methods yet)
specs/         The contract repository — database, api, schemas, events, sdk docs, contracts.
                Read-only reference; nothing here changes it.
tools/         Internal dev tooling, added as it's needed
scripts/       Operational scripts
tests/         Cross-cutting/e2e tests spanning multiple services
docs/          This documentation
```

## What's real right now

- `services/foundation/` — logging, errors, config, id/clock utilities. Go, stdlib only, fully tested.
- `packages/shared-types`, `sdk/typescript` (errors only), `apps/dashboard` (scaffold only) — TypeScript, built and tested.
- CI (`.github/workflows/ci.yml`), lint/format/commit hooks, local Docker infra for Postgres/Redis/NATS.

## What's not here yet, on purpose

Everything domain-specific: the database schema, authentication, the provider registry, the gateway, any adapter. Those are roadmap epics 2 onward, each building on this foundation, none of them started here. See `docs/architecture-references.md` for where each one is already fully specified, waiting to be implemented.
