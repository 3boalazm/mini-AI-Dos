# Repository Overview

## Why Go and TypeScript in one repo

The Technical Design Specification made a firm, deliberately-argued call: Go for the request-serving backend (`services/`), because at this system's stated scale — millions of requests, horizontal scale — goroutine concurrency and static typing matter more than they do at smaller scale. That decision was never in question. What this repository resolves is narrower: the `sdk/` and `apps/` directories are genuinely, separately TypeScript — a client SDK consumers import into their own (often TypeScript) codebases, and a dashboard that's React per the existing project stack. Both were already TypeScript domains before this repository existed; Project Foundation didn't introduce that, it gave it real tooling.

## Where the boundary actually sits

| Directory         | Language          | Why                                                                                                                           |
| ----------------- | ----------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `services/`       | Go                | Request-volume hot path — Gateway, Routing, Adapters, Registry writes                                                         |
| `sdk/typescript/` | TypeScript        | Client library — consumers are typically TS/JS codebases                                                                      |
| `apps/dashboard/` | TypeScript        | React frontend, matching the existing UI stack                                                                                |
| `packages/`       | TypeScript        | Shared config and types _for the TypeScript side_ — not shared with Go, which has its own foundation in `services/foundation` |
| `specs/`          | Language-agnostic | JSON Schema, OpenAPI, Markdown contracts — the source both language sides generate against                                    |

Git hooks (Husky, lint-staged, Commitlint) sit at the repository root and orchestrate both sides — `lint-staged` runs `gofmt` on staged `.go` files and `eslint`/`prettier` on staged `.ts` files from the same pre-commit hook. This is a standard pattern for polyglot monorepos, not a workaround: the tools that operate at the git level don't care what's inside the files they're triggered by.

## Why `services/foundation` has no external dependencies

This is a deliberate engineering stance, not a sandbox limitation dressed up as a principle (though it was also verified against a sandbox that couldn't reach the Go module proxy, which made the stance easy to hold honestly rather than by assertion). A foundation layer that every future service depends on should have the smallest possible dependency surface — every third-party package added here is a package every service inherits, whether it needs it or not. `log/slog`, `crypto/rand`, `encoding/hex`, and the rest of the standard library cover what Project Foundation actually needs. When `services/gateway` or `services/registry` need a Postgres driver or an HTTP router, those are _that service's_ dependencies, declared in _that service's_ `go.mod` — not pulled into the shared foundation every other service would then carry too.

## Why `specs/` is read-only from this repository's perspective

`specs/` (built in an earlier phase of this project) is the canonical contract repository — every JSON Schema, OpenAPI route, and interface contract this codebase implements against. Nothing in `ai-dos/` regenerates or hand-edits it; a future codegen step (turning `specs/schemas/*.schema.json` into TypeScript types or Go structs) reads from it, never writes to it. If a contract needs to change, that change happens in `specs/` first, as its own reviewed decision — not as a side effect of an application code change here.
