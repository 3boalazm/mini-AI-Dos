# Phase 2 — Organization & Project Foundation: Scope Definition

Status: **proposed, not yet approved, not yet implemented.** This document is the design deliverable requested before Phase 1 or Phase 2 begins — no code, schema, or migration exists yet. Written entirely from `specs/`, cited line-by-line below; nothing here is inferred from the roadmap document itself.

## A. Dependency graph (evidence-cited)

```
Organization  ──────────────────────────────────────────────┐
  organization.schema.json required:                        │
  [id, name, billing_email, tier, created_at]                │
  Root of tenancy — not itself organization_id-scoped.       │
                                                               │
      │ organization_id FK (required)                        │ organization_id FK (required)
      ▼                                                       ▼
   Project                                              RoutingPolicy
   project.schema.json required:                        routing_policy.schema.json required:
   [id, organization_id, name, created_at]               [id, organization_id, name,
   team_id, default_routing_policy_id: OPTIONAL           strategy_ref, is_default, created_at]
                                                          Belongs directly to Organization,
      │ project_id FK (required)                          NOT to Project.
      ▼
   APIKey
   api_key.schema.json required:
   [id, project_id, key_prefix, created_by, created_at]
      │
      │ specs/api/authentication.md line 11 (verbatim):
      │ "Maps to | User.identity_provider_subject |
      │           APIKey.project_id → Organization"
      ▼
   Gateway request authentication
   (specs/contracts/gateway-contract.md step 1: "Authenticate
   the caller against an AI-DOS API key") resolves
   organization_id by walking APIKey.project_id → Project.organization_id
      │
      ▼
   Gateway hands off to the Routing Engine
   (specs/contracts/gateway-contract.md step 3), which
   resolves the caller's RoutingPolicy via organization_id
   (routing_policy.schema.json's organization_id, confirmed
   by specs/api/routing.md's route: GET/POST
   /v1/organizations/{id}/routing-policies)
```

**The chain the original roadmap violated:** `Organization → Project → APIKey → (Gateway auth)` and, in parallel, `Organization → RoutingPolicy → (Gateway routing)`. Both legs root at Organization; neither is satisfiable without `Organization` and `Project` existing as real, populatable tables first.

One correction to the problem statement as given: `RoutingPolicy` belongs to **Organization** directly (`routing_policy.schema.json` requires `organization_id`, has no `project_id` field at all), not to `Project`. `Project` only optionally _references_ a `RoutingPolicy` as its default (`project.schema.json`'s `default_routing_policy_id`, nullable — not required). This doesn't change the conclusion (Organization is still the hard root dependency for routing), but it does mean Gateway/Routing's hard schema dependency is `Organization`, not `Organization → Project → RoutingPolicy` as a chain — worth being precise about since it affects what "minimal" means (`RoutingPolicy` needs `Organization` to exist; it does not need `Project.default_routing_policy_id` to be populated to function, since an org-level `is_default=true` policy is enough for a request whose caller's project hasn't set one).

## B. Why the original Phase 2/3 ordering was invalid

Revision 2 placed:

- Phase 2 — Authentication & API Keys
- Phase 3 — Gateway
- Phase 6 — Organization / Project / RBAC

`APIKey` (`api_key.schema.json`) has `project_id` as a **required, non-nullable** field, and `specs/api/authentication.md` states outright that API-key auth's identity resolution _is_ `APIKey.project_id → Organization` — not an incidental relationship, the literal mechanism. `RoutingPolicy` (`routing_policy.schema.json`) has `organization_id` as a **required, non-nullable** field, and `specs/api/routing.md` scopes routing-policy management under `/v1/organizations/{id}/routing-policies`. Neither `Organization` nor `Project` can be deferred to Phase 6 without Phase 2 (Auth) and Phase 3 (Gateway) either (a) inventing a placeholder/fake organization — explicitly prohibited ("Do not create fake implementations... Do not create placeholder implementations and call them complete") — or (b) silently reaching into Phase 6's scope to build the tables anyway, which is the exact kind of undocumented, unplanned scope leakage a phased execution model exists to prevent. Both outcomes are worse than reordering the roadmap. This is a genuine dependency-graph violation, not a stylistic preference.

## C. Proposed corrected sequence (Revision 3)

| #     | Phase                                 | Change from Revision 2                             |
| ----- | ------------------------------------- | -------------------------------------------------- |
| 0     | Foundation Verification               | unchanged                                          |
| 1     | Database Foundation                   | **scope clarified, not enlarged** — see note below |
| **2** | **Organization & Project Foundation** | **new** — minimal slice, this document             |
| 3     | Authentication & API Keys             | unchanged content, renumbered (was 2)              |
| 4     | Gateway & Routing                     | unchanged content, renumbered (was 3)              |
| 5     | Provider Adapter System               | renumbered (was 4)                                 |
| 6     | Usage, Cost & Metrics                 | renumbered (was 5)                                 |
| 7     | Organization & RBAC (Advanced)        | renumbered (was 6, **rescoped** — see D/E)         |
| 8     | SDKs                                  | renumbered (was 7)                                 |
| 9     | Reliability / Observability           | renumbered (was 8)                                 |
| 10    | Production Hardening                  | renumbered (was 9)                                 |
| 11    | Integration / E2E                     | renumbered (was 10)                                |
| 12    | Final Production Readiness            | renumbered (was 11)                                |

**Phase 1 scope note (clarification, not enlargement — Phase 1 itself is not being implemented now):** Phase 1's original Definition of Done said "implement required entities" without specifying which of `specs/database/entities.md`'s ~37 entities that means. Migrating all 37 in one phase, before any consumer of most of them exists, would front-load work against nothing — `docs/architecture-references.md`'s own stated philosophy for this repository is the opposite ("make sure that when epic 2 starts, it's writing a... migration against a repository that already builds... not setting up tooling for the first time under deadline pressure"). The evidence-grounded reading: **Phase 1 owns the migration mechanism** (`golang-migrate` tooling per `specs/database/migrations-strategy.md`, connection pooling, transaction abstraction, the repository-pattern convention, the RLS convention as a reusable pattern) — **not a mandate to pre-create all 37 tables.** Phase 2 is then the first phase to use that mechanism for real tables. This isn't a scope change I'm making unilaterally now (Phase 1 isn't being implemented in this pass); it's a clarification needed so Phase 2's dependency on Phase 1 is coherent — Phase 2 needs Phase 1's _tooling_, not a specific pre-built table.

## D. Scope of Phase 2 — Organization & Project Foundation

**In scope**, each item cited to its schema requirement:

| Item                                                                                                                                                                                                 | Cited requirement                                                                                                                                                                      |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Organization` table: `id` (uuid, PK), `name`, `billing_email`, `tier` (enum `standard`\|`enterprise`), `created_at`, `updated_at`, `deleted_at` (soft delete), `default_region_id` (nullable)       | `organization.schema.json` — all 6 named fields exactly as declared; `required: [id, name, billing_email, tier, created_at]`                                                           |
| `Project` table: `id`, `organization_id` (FK → Organization), `name`, `created_at`, `updated_at`, `deleted_at`, `team_id` (nullable), `default_routing_policy_id` (nullable)                         | `project.schema.json` — `required: [id, organization_id, name, created_at]`                                                                                                            |
| `organization_id` foreign key + Postgres Row-Level Security policy on `Project` (and the reusable RLS pattern every later tenant-scoped table will follow)                                           | `specs/database/relationships.md`: "Every tenant-scoped table carries `organization_id`; an RLS policy restricts every query to rows matching `current_setting('app.current_org_id')`" |
| Minimal repository/data-access layer: create Organization, get Organization by id, create Project, get Project by id, list Projects by organization                                                  | Direct consequence of Phase 1's repository-pattern convention applied to the two new tables; no spec names specific methods beyond what CRUD the schemas' required fields imply        |
| Minimal, internal-only creation path sufficient for Phase 3 (Auth) to create a Project for a new API key to belong to, and for Phase 4 (Gateway) to resolve an `organization_id` from a `project_id` | Direct consequence of the dependency graph in section A — this is the _reason_ the phase exists                                                                                        |

**What "minimal" means concretely**: enough for `INSERT INTO organizations (...)` and `INSERT INTO projects (...)` to succeed against the schema's required fields, and enough for a repository method to answer "what organization does this project belong to" — nothing about workflow, UI, or self-service.

## E. Explicit exclusions from Phase 2

Per your instruction, and cross-checked against `specs/` to confirm none of these are secretly load-bearing for Phase 3/4:

| Excluded                                                         | Where it actually belongs                                                                                                                                                                                                       | Why it's safe to defer                                                                                                                                                                                                         |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Full organization management (update name/tier, org settings UX) | Phase 7 (Organization & RBAC, Advanced)                                                                                                                                                                                         | Phase 3/4 only ever _read_ `organization_id`/`Organization` existence; neither needs update flows                                                                                                                              |
| Billing/subscription/invoice logic                               | Phase 6 (Usage, Cost & Metrics) — `Subscription`/`Invoice` are separate entities in `specs/database/entities.md`'s "Usage and billing domain," unrelated schema-wise to `Organization`/`Project` beyond an `organization_id` FK | `billing_email` is a **field** on the required `Organization` row (schema-mandated), not billing **functionality** — storing the string satisfies the schema; no invoicing/Stripe/metering logic is implied or needed          |
| Quotas                                                           | Not yet defined anywhere in `specs/` at all (no quota field on any schema found)                                                                                                                                                | Nothing in Phase 3/4 references a quota                                                                                                                                                                                        |
| Advanced RBAC (custom roles, fine-grained permissions)           | Phase 7, if ever — **flagged as a real gap**, see H                                                                                                                                                                             | `specs/` only defines a flat 4-role enum (`org_admin`, `registry_admin`, `registry_contributor`, `member`) on `User.role`; nothing "advanced" is currently specified to defer _to_                                             |
| Invitations / membership management                              | Phase 7                                                                                                                                                                                                                         | `User` creation itself belongs to Phase 3 (see next row), not Phase 2 — inviting a user presupposes Phase 3 exists                                                                                                             |
| `Team` entity                                                    | Phase 7                                                                                                                                                                                                                         | `project.schema.json`'s `team_id` is nullable/optional — not a Phase 3/4 dependency                                                                                                                                            |
| `User` table                                                     | Phase 3 (Authentication & API Keys)                                                                                                                                                                                             | `User.identity_provider_subject` (`specs/database/entities.md`) is fundamentally an auth concept; `APIKey.created_by → User.id` is satisfied _within_ Phase 3 since both are introduced there together — not a cross-phase gap |
| Dashboards / admin UX                                            | Later, unscoped in this roadmap so far                                                                                                                                                                                          | No phase in this 13-phase plan currently owns a UI; out of scope everywhere, not just Phase 2                                                                                                                                  |

## F. Definition of Done (for when Phase 2 is actually implemented — not now)

- [ ] `Organization` and `Project` tables exist via a `golang-migrate` migration built on Phase 1's tooling, matching `organization.schema.json` / `project.schema.json` field-for-field (types, nullability, defaults).
- [ ] RLS policy on `Project` enforced and tested (a query without the correct `app.current_org_id` session variable returns zero rows for another org's project, not an error and not all rows).
- [ ] Repository methods for the 5 operations in section D exist and have unit + integration tests against a real Postgres instance (not mocked — this repo's own testing rule, restated in the original mandate: "Integration tests must use a real PostgreSQL instance").
- [ ] `deleted_at` soft-delete honored on reads (a soft-deleted Organization/Project doesn't appear in "get"/"list" results) — per `specs/database/entities.md`'s stated soft-delete convention, which explicitly names `Organization` and `Project` among the entities it applies to.
- [ ] No billing, quota, RBAC-beyond-existence-of-`User.role`-as-a-column, invitation, or UI code introduced.
- [ ] `docs/implementation/PHASE_2_REPORT.md` written with the same evidentiary standard as `PHASE_0_REPORT.md`.

## G. Files/docs changed in this pass

- **New**: `docs/implementation/PHASE_2_SCOPE.md` (this file).
- **Modified**: `docs/implementation/ROADMAP_STATUS.md` — phase table renumbered to Revision 3 (13 phases), revision history entry added, Phase 1's scope note added.
- **Not changed**: no source code, no migrations, no schema, nothing under `services/` or `specs/`. Nothing committed — both files are currently uncommitted working-tree changes, per your explicit instruction to hold the commit until you've reviewed this.

## H. Remaining dependency risks (not resolved here, flagged for your review)

1. **"Advanced RBAC" (Phase 7) may be a near-empty phase as specs/ currently stands.** Only a flat 4-role enum exists anywhere in `specs/`; there's no fine-grained permission model to build "advanced" RBAC out of. Either Phase 7 stays thin (just Team + org-management UX + invitations), or a permission model needs to be spec'd first — a `specs/` gap, not a roadmap gap.
2. **`User` sits at a phase boundary.** It's created in Phase 3 (Auth) per this document's reasoning, but it's also `organization_id`-scoped like every Phase 2 entity. If Phase 3 is ever implemented by someone other than whoever did Phase 2, the `organization_id` FK/RLS convention needs to be documented clearly enough to be followed without re-deriving it — `PHASE_2_REPORT.md` (section F above) should make that convention explicit when it's written, not just implicit in the migration file.
3. **Phase 1's scope note (section C) is a clarification this document makes, not a decision Phase 1 itself has ratified**, since Phase 1 hasn't been implemented or re-scoped yet. When Phase 1 actually starts, its own audit/design step should confirm this "tooling, not full-schema" reading still holds — I'm flagging it as the reading that makes Phase 2 coherent, not asserting it's been separately approved for Phase 1 on its own terms.
4. **`default_region_id` on `Organization`** (`organization.schema.json`) references a `Region`/`ProviderRegion` concept (`specs/database/entities.md`'s Provider domain) that belongs to a much later phase (Provider Adapter System, Phase 5). It's nullable, so Phase 2 can leave it `null` unconditionally — but worth naming so it isn't mistaken for a Phase 2 dependency in the other direction (Organization depending on something from Phase 5) when it's actually just an optional, always-nullable-for-now column.

**Stopping here, as instructed. Not starting Phase 1 or Phase 2 until you review and explicitly approve this revised roadmap.**
