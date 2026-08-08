# Phase 1 — Database Foundation: Scope Determination

Status: **decision reached, not yet approved, not yet implemented.** No migration, schema, or application source code exists yet as a result of this document — this is the planning/scope gate requested before Phase 1 implementation begins.

## A. Canonical evidence for Phase 1 scope

**Searched first for the most direct evidence — and found it's missing.** `docs/architecture-references.md` and `docs/repository-overview.md` both cite an "implementation roadmap (`00-roadmap-overview.md`, `01-sprint-1-detailed-tasks.md`)" and a "Technical Design Specification" as the source for "every engineering epic, task, and the dependency graph between them" and "technology choices and why." Checked via `git ls-files` and a full-text `git grep` across every tracked file: **neither `00-roadmap-overview.md`, `01-sprint-1-detailed-tasks.md`, nor any TDS document exists anywhere in this repository's tracked history.** They are dangling citations — referenced, never included. Per the standing rule this whole engagement operates under (the repository is the only source of truth, prior claims are not evidence), I cannot use their presumed contents as evidence for anything, including Phase 1's scope. This is itself a finding, not just a dead end — see H.

With that ruled out, the actual canonical evidence:

1. **`specs/database/migrations-strategy.md`** (read in full this pass) — entirely about _mechanism_: `golang-migrate`, forward-only, one file per change, `schema_version` semver, expand-then-contract sequencing, partition-creation-is-not-a-migration, down-migrations-exist-but-aren't-auto-run. **Zero mention of which entities to migrate or when.** Entity-agnostic by construction.
2. **`specs/database/erd.md`** (read in full this pass) — presents all ~37 entities as two diagrams, explicitly split _only_ for diagram readability ("one combined diagram... would be unreadable"), not as an implementation-sequencing boundary. Describes the _target_ schema as a whole, not a phased build order.
3. **`specs/database/entities.md`, `specs/database/relationships.md`** (read in full earlier this session) — same character: complete target-state field tables and cross-cutting policy (RLS, partitioning, normalization), no phasing information.
4. **`services/foundation/errors/errors.go`'s own doc comment** (already-implemented code, not a spec, but a demonstrated precedent from this exact codebase): _"This package is deliberately generic. It does NOT define `ProviderError`... Defining them here would mean epic 1 reaching into epic 5's scope before epic 5 exists to own it."_ This is the one place in the repository where the project's own phasing discipline is stated explicitly, in the project's own words, by whoever built Phase 0. It's the closest thing to a phasing _principle_ that exists anywhere in this codebase.
5. **The now-approved `PHASE_2_SCOPE.md`** — establishes, with schema-level citations, that `Organization` and `Project` are Phase 2's to create. This is a hard constraint on Phase 1's scope now, not just supporting evidence: whatever Phase 1 turns out to own, it cannot also claim `Organization`/`Project`, or Phase 1 and Phase 2 duplicate the same work.

**Conclusion on the A/B/C question, from this evidence**: `specs/database/*` describes _what the finished house looks like_, not _which room gets built first_ — that sequencing question is answered by this repository's own demonstrated convention (#4) and by hard dependency necessity (which later phase needs which table), not by any single spec file. Neither pure "tooling only, zero schema content" nor "the full 37-entity schema" is directly supported; both undershoot or overshoot the evidence.

## B. Exact Phase 1 responsibility

**Interpretation C — a specific, narrow, dependency-justified subset — but the subset is _mechanism made real_, not _the first domain tables_.** Concretely, Phase 1 owns:

1. **PostgreSQL integration**: connection pooling, configuration (extending `services/foundation/config.Loader` — already generic and ready for this per its own doc comment — not a new competing config mechanism).
2. **Migration tooling**: `golang-migrate` wired up and runnable, per `specs/database/migrations-strategy.md`'s stated tooling choice.
3. **A generic repository pattern/convention**: an interface shape (Go generics or an equivalent convention) that Phase 2 onward implements per-entity — not a per-entity implementation itself.
4. **The reusable, entity-agnostic conventions every future table follows**: the RLS session-variable pattern (`current_setting('app.current_org_id')`, per `specs/database/relationships.md`), the soft-delete (`deleted_at`) convention, the `schema_version` column convention, UUIDv7 primary keys (already implemented, `services/foundation/util.NewUUIDv7`) — demonstrated and tested against the tooling, not against a real domain entity.
5. **Transaction abstraction** and **integration-test harness** (real Postgres, not mocked — see G) proving the above actually works end-to-end.

**What Phase 1 does _not_ create: any table from `specs/database/entities.md`.** Not `Organization`, not a scratch/placeholder table pretending to be one, not any of the ~37 real entities. The integration tests in item 5 prove the mechanism (a migration applies to an empty database, is repeatable, a transaction rolls back correctly, a repository generic pattern round-trips against a trivial/throwaway table structure created and dropped within the test itself, RLS actually restricts cross-tenant rows on a test fixture) — without pre-building any entity that a later phase is scoped to own.

## C. Explicit Phase 1 exclusions

- **`Organization`, `Project`** — Phase 2's, per the already-approved `PHASE_2_SCOPE.md`. This is the constraint that most directly shapes this answer (A/5).
- **`User`, `APIKey`** — Phase 3's (Authentication & API Keys), per the dependency graph in `PHASE_2_SCOPE.md` section A.
- **`RoutingPolicy`, `RoutingRule`, `RoutingHistory`** — Phase 4's (Gateway & Routing).
- **`Provider`, `Model`, `ModelVersion`, `Capability`, `ProviderPricing`, `ProviderLimits`, `ProviderRegion`** — Phase 5's (Provider Adapter System); explicitly "platform-global," not tenant-scoped, per `specs/database/entities.md`'s own conventions section — a structurally different kind of table Phase 1's tenant-aware conventions don't even apply to.
- **`UsageRecord`, `Invoice`, `Subscription`** — Phase 6's (Usage, Cost & Metrics).
- **`Team`** — Phase 7's (Organization & RBAC, Advanced), per `PHASE_2_SCOPE.md` section E.
- **Everything else in `specs/database/entities.md`** (Benchmark\*, Health\*, VersionHistoryEntry, Request domain, Content domain, Agent domain, Platform domain) — see H for the two of these that don't currently have _any_ owning phase, which is a roadmap gap, not a Phase 1 scope question.

## D. Dependency justification for Phase 2

Phase 2 can safely depend on Phase 1 because Phase 1 delivers a **working, integration-tested mechanism** — not a promise, not a stub. Phase 2's own Definition of Done (`PHASE_2_SCOPE.md` section F) requires a real migration, a real RLS policy, and real-Postgres integration tests for `Organization`/`Project`; all three require the tooling, connection handling, and conventions Phase 1 is scoped to prove out first. This is the same shape of dependency `docs/architecture-references.md` describes between Project Foundation (Phase 0) and "epic 2": Phase 0 didn't write any database code, but made sure the repository "already builds, lints, and tests cleanly" before database work started "under deadline pressure." Phase 1 is that same relationship one level down — it doesn't write `Organization`, but makes sure the migration/repository/RLS mechanism is proven before Phase 2 writes its first real one.

## E. Phase 1 Definition of Done

Supersedes the original Phase 1 Definition of Done's ambiguous items (kept where unambiguous, sharpened where not):

- [ ] `docker compose up -d` (this repo's own `docker-compose.yml`) yields a reachable PostgreSQL 16 instance; connection pooling from Go code confirmed against it.
- [ ] `golang-migrate` runs against an **empty** database and succeeds — no entity migration required to satisfy this, since `golang-migrate` manages its own bookkeeping table independent of any domain schema.
- [ ] Migrations are demonstrated repeatable (apply → down → apply again, clean) using a throwaway/example migration exercised only inside the test suite, not shipped as a real entity migration.
- [ ] The generic repository pattern is implemented and has unit tests independent of Postgres (business logic testable without infrastructure, per the standing cross-phase invariant).
- [ ] The RLS convention (session-variable-scoped tenant isolation) is implemented and integration-tested against a real Postgres instance with a throwaway fixture table — proving a query without the correct `app.current_org_id` session variable returns zero rows, before any real tenant-scoped entity exists to prove it on for real.
- [ ] Transaction abstraction (begin/commit/rollback) integration-tested against real Postgres.
- [ ] No hardcoded DB credentials — `services/foundation/config.Loader` (already implemented) is the seam used, consistent with Phase 0's own established pattern.
- [ ] `docs/implementation/PHASE_1_REPORT.md` written with the same evidentiary standard as `PHASE_0_REPORT.md`.
- [ ] **Explicitly not required for Phase 1's gate**: any of `specs/database/entities.md`'s ~37 entities existing as a real migration.

## F. Required infrastructure/prerequisites

1. **A reachable PostgreSQL instance** — hard requirement, see G. This machine's `docker-compose.yml`-provisioned Postgres is still unreachable (re-confirmed this pass: `wsl --status` still reports WSL not installed, unchanged since the last report).
2. **`golang-migrate` CLI/library** — not yet installed on this machine; a `go install` of the CLI (or importing the library) is a normal Go-tooling action, same category as the `golangci-lint` install already done in Phase 0 — not flagged as needing separate permission, but not yet done since implementation hasn't started.
3. **Go toolchain** — already verified working (Phase 0).
4. **No new external services beyond Postgres** — Redis and NATS (also in `docker-compose.yml`) are not Phase 1 dependencies; nothing in this scope touches caching or eventing.

## G. Acceptance-test strategy — is real PostgreSQL required?

**Yes, unambiguously, for two independent reasons:**

1. The Definition of Done items in E (migrations against an empty database, repeatability, RLS actually restricting rows, transaction rollback behavior) are **infrastructure behavior**, not business logic — they cannot be meaningfully verified any other way. A mock migration runner or a mocked Postgres driver would verify that the mock behaves as programmed, not that a migration actually runs; per the standing rule from this engagement's own mandate, that's exactly the "mocked production implementation" / "replace integration tests with mocks just to make the phase green" pattern that's explicitly prohibited.
2. The more recent explicit instruction's own **DATABASE RULE** states outright: "Integration tests must use a real PostgreSQL instance" — not a preference, a requirement, for every database phase.

Unit tests (the generic repository pattern's interface behavior, config-loading logic) can and should run without Postgres, same as `services/foundation`'s existing tests do. But the phase's gate-defining checks — the ones in E marked "integration-tested against real Postgres" — cannot pass, and must not be faked, without one.

## H. Roadmap/documentation adjustments identified

**Made in this pass** (uncommitted, per instruction):

- This file (`PHASE_1_SCOPE.md`), new.
- `docs/implementation/ROADMAP_STATUS.md` — Phase 1's status note updated to reference this scope determination (see the file itself for the exact wording).

**Identified but _not_ acted on this pass — flagged for your review, since resolving them wasn't this pass's scope**:

1. **The `AgentSession`/`AgentMemory` entities (`specs/database/entities.md`'s "Agent domain") have no owning phase anywhere in Revision 3.** Revision 2 deliberately dropped every agent-runtime phase (Revision 1's phases 5–9) because nothing in `specs/` defines an agent runtime, tool runtime, or orchestration layer — but the _database_ spec still names two agent-shaped tables nobody in the current 13-phase roadmap is scoped to build. Either they're a genuine future gap (a 14th phase not yet named) or they're a leftover from whatever originally motivated Revision 1 and should be marked explicitly out of scope in `specs/database/entities.md` itself. Not resolved here — this is a `specs/` question, and `specs/` is stated to be immutable from this repository's side without its own reviewed decision.
2. **No phase in Revision 3 owns eventing/NATS integration**, despite `specs/events/*.md` (5 files) fully specifying topics/envelopes and `docker-compose.yml` already provisioning NATS JetStream. `EventLog` (Platform domain) is the table most directly affected — it has no obvious phase home either. Smaller, related gap: `specs/database/entities.md`'s Request domain (`RequestLog`, `StreamingSession`, `ToolCall`) and Content domain (`PromptTemplate`, `PromptCache`) also don't map cleanly onto a named phase, though they're less urgent (they're natural fits near Phase 9, Reliability/Observability, just not stated).

Neither of these blocks Phase 1 (Phase 1 excludes all of these entities regardless of which future phase eventually claims them — see C). Raising them now because they surfaced directly while tracing "what does each phase own," not as scope creep on this question.

## I. Final recommendation

**NOT READY TO IMPLEMENT.**

The scope question this document was asked to resolve is answered (B/C/E above), and Phase 1 is well-defined enough to implement once unblocked. But G is unambiguous: Phase 1's gate requires a real PostgreSQL instance, and re-checked fresh this pass, `wsl --status` still reports WSL not installed — the same restart boundary from the original Phase 1 blocker, unchanged. Implementation should not start until either that's resolved, or you direct a different path (native PostgreSQL without Docker was offered earlier as an alternative and not chosen; still available if you'd rather not deal with WSL2 at all).

**Stopping here, as instructed.** Waiting on your review of this scope determination before any Phase 1 code, migration, or commit.
