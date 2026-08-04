# AI-DOS — As-Built Architecture

This document reflects **only what is actually implemented and verified in this repository**, updated at the end of every phase. It is not a design doc — `specs/` is the design reference this implementation builds against. If this file and `specs/` ever disagree about what's _built_, this file is describing reality and `specs/` is describing intent; if implementation and spec disagree about what _should_ be built, that's a Phase acceptance-gate blocker, not something to silently resolve here.

Last updated: end of Phase 0.

## Current system shape

```
specs/ (43 files — read-only design reference, not running code)
   │
   ▼
services/foundation (Go, stdlib-only)     packages/shared-types, sdk/typescript, apps/dashboard (build-pipeline stub)
   │                                           │
   ▼                                           ▼
(nothing imports it yet — no other Go module exists)   (nothing beyond error types/build-info exists)
```

No API layer, no client, no database connection, no event consumer/publisher exists anywhere yet. This is expected at Phase 0 — the foundation phase intentionally ships no domain logic.

## Implemented components

### `services/foundation` (Go 1.22 module `github.com/ai-dos/foundation`, stdlib-only)

| Package   | Responsibility                                  | Public surface                                                                                               |
| --------- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `config`  | Typed env loading                               | `Loader`, `New()`, `NewFromMap()`, `RequireString()`, `OptionalString()`, `OptionalInt()`, `MissingEnvError` |
| `errors`  | Generic app error type + RFC 7807 serialization | `AppError`, `Code` (6 generic codes), `New()`, `Wrap()`, `ToProblemDetails()`                                |
| `logging` | Structured logging over `log/slog`              | `Logger`, `Config`, `New()`, `WithTraceID()`, `FromContext()`                                                |
| `util`    | UUIDv7 + testable clock                         | `NewUUIDv7()`, `Clock`, `RealClock`, `FakeClock`                                                             |

Not a runnable service (no `main.go`). Every future Go service is expected to import this module via `go.work`'s `use` directives as those modules are created — none exist yet.

### TypeScript workspace (pnpm + turbo monorepo)

| Package                 | Responsibility                                               | Public surface                                                                                                      |
| ----------------------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------- |
| `packages/shared-types` | Infrastructure types, not domain entities                    | `Result<T,E>`, `ok`/`err`/`isOk`/`isErr`/`map`/`unwrapOr`, `PaginationParams`, `PaginatedResult<T>`, `clampLimit()` |
| `sdk/typescript`        | Client SDK error hierarchy only                              | `AIDosError` + 5 subclasses, mirrors `services/foundation/errors` field-for-field                                   |
| `apps/dashboard`        | Proves the build/lint/test pipeline works for an app package | `getBuildInfo()` — not a real dashboard; no UI framework dependency exists                                          |

## Explicitly not implemented (any phase, any component)

Database layer, authentication/SSO/RBAC, tenancy, gateway, routing, provider adapters, agent runtime, tool runtime, memory/vector store, orchestration/subagents, channels/voice, production observability. Each is scoped to a specific later phase in `docs/implementation/ROADMAP_STATUS.md` and must not be assumed to exist until that phase's report says so.

## Cross-phase architectural invariants (from the execution mandate, restated here so they stay visible)

- Services own their data; no cross-service DB foreign keys.
- Provider-specific behavior lives only in adapters, never in core/gateway code.
- Configuration is externalized (the `config` package's `Loader` seam, extended per-service).
- Business logic must be testable independent of infrastructure (the `Clock`/`NewFromMap` pattern established in Phase 0 is the model to keep following).
- `specs/` is immutable from application code's side — a spec/implementation disagreement is a blocker to resolve explicitly (see Phase 0's two spec fixes below for the one class of exception: fixing an objectively broken _spec-internal_ inconsistency, not changing a spec to match convenient implementation).

## Phase 0 changes to `specs/` (spec-internal fixes, not new implementation)

1. `specs/schemas/_verify_all.py` — was hardcoded to glob `/home/claude/specs/schemas/*.schema.json` (a leftover from the original authoring sandbox). Outside that exact path it silently matched zero files and still printed `ALL CHECKS PASSED` — a false positive, not a real check. Fixed to resolve the schema directory relative to the script's own location via `os.path.dirname(os.path.abspath(__file__))`.
2. `specs/api/openapi.yaml` — `specs/api/providers.md` documents `POST /internal/v1/providers/{id}/deprecate` (OIDC `registry_admin`, `reason` required), matching the `published → deprecated → archived` lifecycle in `specs/contracts/provider-contract.md`, but the path was entirely absent from `openapi.yaml`'s `paths:` (its sibling `/internal/providers/{id}/publish` existed, `/internal/providers/{id}/deprecate` didn't). Added, modeled directly on the `publish` operation, with a required-`reason` request body per `providers.md`'s own stated requirement.

## Phase 0 code fix

`services/foundation/errors/errors_test.go`'s `TestToProblemDetails_NeverLeaksCause` discarded a `json.Marshal` error (`body, _ := json.Marshal(pd)`). Harmless in practice (`pd` is always-marshalable plain data) but a real `golangci-lint` `errcheck` failure under this repo's own `check-blank: true` setting — and CI's `golangci-lint-action` step has no tolerance for it, unlike the `Makefile`'s `lint` target which suffixes `|| true`. Fixed to check-and-`t.Fatalf`, matching the pattern the very next test in the same file already uses.

## Toolchain notes for whoever runs this next

- Go 1.26.5 installed via `winget install GoLang.Go` (repo's `go.mod`/CI pin `go 1.22` — newer toolchain building an older-declared module works fine; no issue observed for AI-DOS's own code).
- `golangci-lint`: CI pins `v1.61.0`, which fails to **compile** under Go 1.26.5 (`golang.org/x/tools@v0.24.0` hits `invalid array length -delta * delta` — a real Go-toolchain/x-tools incompatibility, not an AI-DOS issue). Used `v1.63.4` (last v1.x release, same `.golangci.yml` schema) instead. Worth bumping CI's pin at some point; out of scope for a spec/app-code-only Phase 0.
- `-race` could not be exercised: this machine has no C compiler, and `-race` requires CGO. `go test ./services/foundation/...` (no `-race`) passes; the CI-equivalent race-enabled run is unverified locally. Flagging rather than silently dropping `-race` from the record.
- Python and Docker are not installed on this machine as of Phase 0; both were offered and explicitly declined for this pass (see `EXECUTION_STATE.md`). Docker becomes a hard requirement starting Phase 1.
