# Authentication

Two mechanisms, two audiences, never accepted on the same route.

| | Humans (Dashboard, admin) | Programmatic callers (Gateway) |
|---|---|---|
| Mechanism | OIDC via Keycloak | AI-DOS-issued API key |
| Header | `Authorization: Bearer <oidc_token>` | `Authorization: Bearer <api_key>` |
| Format | JWT | `aidos_{env}_{random32}` |
| Storage | Never stored — Keycloak session | `key_hash` only (`../schemas/api_key.schema.json`); raw key returned exactly once, at creation |
| Maps to | `User.identity_provider_subject` | `APIKey.project_id → Organization` |

**Authorization (RBAC)**: `org_admin`, `registry_admin`, `registry_contributor`, `member` (`User.role`). A `registry_contributor` submitting an `unverified`-trust plugin cannot self-publish regardless of role — trust level and role are independent gates, both required. See `../contracts/validation-rules.md`.

**Multi-tenant enforcement**: `organization_id` set as a Postgres session variable from the resolved identity before any query runs; Row-Level Security refuses cross-tenant rows at the database, not only in application code. See `../database/relationships.md`.

**Key rotation**: a new key is issued alongside the old one (never in-place replacement); the old key is revoked (`revoked_at` set, never deleted) once traffic has migrated — confirmed via `last_used_at`, not assumed.
