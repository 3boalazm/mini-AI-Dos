# Architecture References

This file indexes where things are already specified. It does not restate them — every contract below was already written, reviewed, and (for the schemas and OpenAPI spec) verified before this repository existed. Duplicating that content here would just create a second copy to keep in sync.

| Looking for...                                                      | Find it in                                                                             |
| ------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| Database entities, relationships, ERDs                              | `specs/database/`                                                                      |
| REST API routes, request/response contracts                         | `specs/api/openapi.yaml`, `specs/api/*.md`                                             |
| Canonical data schemas (Provider, Model, Organization, ...)         | `specs/schemas/*.schema.json`                                                          |
| Async event contracts (producer, consumer, retry, DLQ)              | `specs/events/*.md`                                                                    |
| SDK interface — the shape `sdk/typescript` implements against       | `specs/sdk/*.md`                                                                       |
| Provider/adapter/gateway contracts                                  | `specs/contracts/*.md`                                                                 |
| Technology choices and why (Go, Postgres, NATS, and so on)          | Technical Design Specification                                                         |
| Every engineering epic, task, and the dependency graph between them | The implementation roadmap (`00-roadmap-overview.md`, `01-sprint-1-detailed-tasks.md`) |

## What this repository's Project Foundation implements against, specifically

- `services/foundation/errors` mirrors `specs/contracts/validation-rules.md`'s error-code-to-HTTP-status mapping structurally (the `Code`-to-status map), but does **not** yet include the mapping table's provider-specific rows (`RateLimitedError` → 429, and so on) — those belong to roadmap epic 5/6, not epic 1. See the package's own doc comment for why.
- `sdk/typescript/errors.ts` mirrors the same generic subset, for the same reason, on the TypeScript side.
- `packages/shared-types` implements infrastructure types (`Result`, pagination) that `specs/` assumes exist but doesn't itself define, since `specs/` describes domain contracts, not language-level utility types.

## What deliberately isn't implemented yet

Every domain entity in `specs/schemas/` (Provider, Model, and the rest), every route in `specs/api/openapi.yaml`, every adapter in `specs/contracts/adapter-contract.md` — all fully specified, none implemented here. Project Foundation's job was to make sure that when epic 2 starts, it's writing a Postgres migration against a repository that already builds, lints, and tests cleanly — not setting up tooling for the first time under deadline pressure.
