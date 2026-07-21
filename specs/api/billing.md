# Billing and Usage API

| Route | Method | Auth | Notes |
|---|---|---|---|
| `/v1/organizations/{id}/usage` | GET | OIDC | Aggregated summary only — never raw per-request `UsageRecord` rows over this API |
| `/v1/organizations/{id}/invoices` | GET | OIDC, `org_admin` | `Invoice` list, Stripe-backed |

**Usage Service versus Cost Optimization, restated because the API surface is where the distinction actually matters to a caller**: this endpoint reflects what was metered *after* a request completed (`UsageRecord`) — it is never derived from a routing-time cost estimate. A caller building their own cost dashboard against this API is reading billing fact, not a prediction, even though a different part of the system (the Routing Engine) makes cost predictions using the same `ProviderPricing` data for a different purpose entirely.

Full field contracts: `UsageRecord`, `Invoice`, `Subscription` — `../database/entities.md`.
