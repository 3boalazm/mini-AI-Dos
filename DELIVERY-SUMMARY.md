# Sprint 1 — Task Group 1: Project Foundation — Delivery Summary

Every command below was actually run in this session, not assumed. Full output is in the conversation; this is the reference summary the task requirements asked for explicitly.

## Folder tree

```
ai-dos/
├── .editorconfig, .env.example, .gitignore, .prettierrc.json, .prettierignore, .golangci.yml
├── .husky/                    pre-commit (lint-staged), commit-msg (commitlint)
├── .github/workflows/ci.yml   two jobs: go, typescript
├── Makefile                   install, build, lint, typecheck, test, fmt, up/down, verify
├── README.md
├── commitlint.config.js, .lintstagedrc.json
├── docker-compose.yml         Postgres 16, Redis 7, NATS JetStream — infra only, no service containers yet
├── go.work                    workspace file; only services/foundation today
├── package.json, pnpm-workspace.yaml, pnpm-lock.yaml, turbo.json
├── apps/dashboard/            foundation scaffold — proves the pipeline, not the real dashboard (epic 15)
├── packages/
│   ├── tsconfig-base/         base.json (strict), library.json (declarations + composite)
│   ├── eslint-config/         shared flat config
│   └── shared-types/          Result<T,E>, cursor pagination — infrastructure types, not domain entities
├── services/foundation/       logging, errors, config, util — Go, stdlib only, zero external deps
├── sdk/typescript/            AIDosError hierarchy only — no API-calling client methods yet
├── specs/                     the existing, already-verified contract repository (copied in, untouched)
├── tools/, scripts/, tests/e2e/   placeholders, populated as later epics need them
└── docs/                      developer-setup, repository-overview, contributing, architecture-references
```

## File list

73 tracked source files outside `specs/` (which contributes its own already-documented 43), 124 including generated lockfiles — the exact count is reproducible: `find . -type f -not -path './node_modules/*' -not -path './.git/*' -not -path './specs/*' | wc -l` from the repo root.

By area: Go foundation — 9 files (4 packages × source + test, plus `go.mod`). TypeScript — 24 files across `apps/dashboard`, `packages/*`, `sdk/typescript`. Tooling/config — 15 files at the root. Docs — 5 files (root `README.md` + 4 in `docs/`). CI — 1 workflow file.

## Dependency graph

```
packages/tsconfig-base  ──┐
packages/eslint-config  ──┼──▶ packages/shared-types ──▶ (future services' TS tooling)
                           │
                           ├──▶ apps/dashboard (scaffold)
                           └──▶ sdk/typescript

services/foundation  ──▶ (every future Go service: gateway, registry, routing, ...)

specs/  ──▶  (read-only; every future package/service implements against it, none of them regenerate it)
```

`services/foundation` and the TypeScript `packages/*` do not depend on each other — this is intentional, not an oversight. The only thing that will eventually cross the language boundary is generated code (TypeScript types and Go structs both generated from `specs/schemas/*.schema.json`), and that codegen step doesn't exist yet because no epic needs it yet.

## Build instructions

```bash
git clone <repo-url> ai-dos && cd ai-dos
make install    # pnpm install — TypeScript dependencies
make up         # docker compose up -d — Postgres, Redis, NATS
make verify     # install + lint + typecheck + build + test, Go and TypeScript both
```

Go requires 1.22+, already on `PATH`, no separate install step — `go build ./services/foundation/...` works directly (note: **not** `go build ./...` from the repo root; see `docs/developer-setup.md` for why that specific command fails and what to use instead).

## Verification checklist

Every line below was actually executed in this session:

- [x] `go vet ./services/foundation/...` — clean
- [x] `gofmt -l services/foundation/` — no output (fully formatted)
- [x] `go build ./services/foundation/...` — succeeds
- [x] `go test -race -cover ./services/foundation/...` — **23 tests, all passing**, coverage 84.2–96.2% across the four packages
- [x] `pnpm install` — 318 packages resolved, workspace links correct
- [x] `pnpm run typecheck` (tsc --noEmit, 3 packages) — clean, after fixing a real `rootDir` resolution bug in the shared base config
- [x] `pnpm run lint` (eslint, 3 packages) — clean
- [x] `pnpm run build` (tsc -b, 3 packages) — succeeds
- [x] `pnpm run test:coverage` (vitest, 3 packages) — **20 tests, all passing**, 92–100% coverage on every file with real logic
- [x] `prettier --check .` — clean, repo-wide, after fixing 3 files and adding `.prettierignore` to stop it flagging generated output
- [x] `docker-compose.yml` — valid YAML, confirmed with a real parser; **not** verified to actually run (no Docker daemon in this environment) — stated as a limit, not glossed over
- [x] `.github/workflows/ci.yml` — valid YAML, structurally checked (2 jobs, correct step counts); **not** run on real GitHub Actions infrastructure from here
- [x] `git init`, full commit history, clean working tree matching the file list above exactly
- [x] Husky hooks installed and executable (`chmod +x` confirmed, not assumed)

## What's explicitly not here, per the task's own exclusion list

Database, authentication, providers, gateway, router, OpenAI adapter, business logic, endpoints, API calls, AI integrations — none implemented, matching the instruction exactly. The Go error foundation and the TypeScript SDK error hierarchy are both deliberately generic (`NotFoundError`, `ValidationError`, and so on) rather than the provider-specific taxonomy (`RateLimitedError`, `AuthError`) `specs/contracts/adapter-contract.md` defines — that taxonomy is roadmap epic 5/6's scope on both language sides, not epic 1's, and building it now would have been implementing ahead of where this task group's boundary actually sits.
