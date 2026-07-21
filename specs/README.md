# specs

The official reference for this project. Anything built after this point — any service, any SDK, any AI working from this repo — builds directly from these files, not from inference over long-form documents.

## Layout

```
specs/
├── database/     entities, relationships, ERDs, migration policy
├── api/          openapi.yaml (validated) + per-domain route docs
├── schemas/      canonical JSON Schemas — the source of truth for every entity's shape
├── events/       every async event: producer, consumers, payload, retry/DLQ, ordering
├── sdk/          one canonical client interface, seven language bindings
└── contracts/    the provider, gateway, and adapter contracts, plus validation policy
```

## Where to start, depending on what you're building

| You're building... | Start with |
|---|---|
| A new provider adapter | `contracts/provider-contract.md` (data) + `contracts/adapter-contract.md` (code) |
| A backend service that reads/writes providers or models | `database/entities.md`, `schemas/` |
| A frontend or dashboard | `api/openapi.yaml`, `sdk/typescript.md` |
| An SDK in a new language | `sdk/README.md` — one canonical shape, don't invent a new one |
| A consumer of async events | `events/` — read the retry/DLQ/ordering columns before assuming at-most-once |
| Anything touching auth or tenancy | `api/authentication.md`, `database/relationships.md` |

## What's complete versus what's a known next step

**Complete and verified**: all 13 files in `schemas/` (well-formed, no `$id` collisions); `api/openapi.yaml` (valid OpenAPI 3.1, including local `$ref` resolution — caught and fixed three real errors during validation, not assumed correct); every entity in `database/entities.md` has a full field-level spec.

**Known, stated gap, not a silent omission**: ~24 entities in `database/entities.md` are fully specified at the field-table level but not yet split into standalone files in `schemas/` — see `schemas/README.md` for the exact list. The pattern for turning one into the other is demonstrated four times over in the files that already exist there.

## Provenance

This repository consolidates work from architecture through technical design through contract specification. Where a file states a design decision, it's citing where that decision was made, not re-deriving it — cross-references throughout (`"see ../schemas/provider.schema.json"`, `"turn 4"`, `"TDS §8"`) point at that provenance rather than repeating it.
