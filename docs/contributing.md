# Contributing

## Branch strategy

Trunk-based, per TDS §11 — short-lived feature branches off `main`, merged via PR once `make verify` passes locally and CI confirms it. No long-lived environment branches (`develop`, `staging`) to keep in sync; anything not ready for `main` stays behind a feature flag rather than a branch that drifts.

## Commit messages

Conventional Commits, enforced by the `commit-msg` hook (Commitlint) — a malformed commit message is rejected locally, before it ever reaches CI. Format: `type(scope): description`.

**Scopes** (matching the top-level workspace areas): `foundation`, `apps`, `packages`, `services`, `sdk`, `specs`, `tools`, `ci`, `docs`.

**Types**: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `ci`, `build`.

Example: `feat(services): add structured logging foundation`

## Before opening a PR

```bash
make verify
```

This runs the exact sequence CI runs. A PR that fails CI after `make verify` passed locally indicates an environment difference worth its own investigation — not a reason to just re-push and see if CI is flaky.

## Code review expectations

- Every file should build, lint, and have real test coverage — the same bar Project Foundation held itself to (`go test -cover`, `vitest run --coverage`), not a lower one for "just this once."
- A PR touching `services/foundation` (or its TypeScript equivalent, the shared packages) gets a second reviewer by default — this layer is imported everywhere, so a mistake here is expensive in a way a mistake in one service isn't.
- Contracts in `specs/` are immutable from this repository's side (see `docs/repository-overview.md`). A PR that needs a contract to be different is a signal to raise that as its own decision, not to quietly work around it in application code.
