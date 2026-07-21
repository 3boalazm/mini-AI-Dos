# Routing API

| Route | Method | Auth | Notes |
|---|---|---|---|
| `/v1/organizations/{id}/routing-policies` | GET | OIDC | List named strategies for an org |
| `/v1/organizations/{id}/routing-policies` | POST | OIDC | Create; `is_default=true` is enforced unique per org at the database level, not just application logic |

A `RoutingPolicy` names a `SelectionStrategy` plugin (`strategy_ref`, never hardcoded) plus an optional `cost_ceiling`. Per-tenant overrides live in `RoutingRule`, layered on top rather than forking the policy. Actual routing *decisions* (`RoutingHistory`) are not exposed as a query-by-request-id API at this volume — they're sampled into traces, per `../database/entities.md`'s Request domain design decision, and reachable through observability tooling, not this API.

Full field contracts: `../schemas/routing_policy.schema.json`; `RoutingRule`/`RoutingHistory`: `../database/entities.md`.
