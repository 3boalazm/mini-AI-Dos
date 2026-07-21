# Providers API

Public catalog read, internal-only write — see `../contracts/gateway-contract.md` for why the boundary sits exactly there.

| Route | Method | Auth | Notes |
|---|---|---|---|
| `/v1/providers` | GET | API key | Full catalog, filterable by `tier`, `compliance_tags`, `status` |
| `/v1/providers/{id}/health` | GET | API key | Current `HealthScore` snapshot — denormalized, not the full signal history |
| `/v1/providers/{id}/incidents` | GET | API key | External-sourced `HealthSignal`s only, `vendor_claim`-tagged, never blended with AI-DOS's own measurements |
| `/internal/v1/providers` | POST | OIDC, `registry_admin` \| `registry_contributor` | Enters at `Draft` — never live on creation |
| `/internal/v1/providers/{id}/publish` | POST | OIDC, `registry_admin` | Runs the schema/uniqueness/`adapter_ref`/smoke-test gate; `registry_admin` required outright if `trust_level=unverified` |
| `/internal/v1/providers/{id}/deprecate` | POST | OIDC, `registry_admin` | `reason` required |

Full field contract: `../schemas/provider.schema.json`. Full entity spec including `ProviderPricing`/`ProviderLimits`/`ProviderRegion`: `../database/entities.md`.
