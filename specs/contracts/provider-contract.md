# Provider Contract

The **data** contract — what a provider registration must satisfy to exist in this system. For the **code** contract — what a class implementing a provider must satisfy — see `adapter-contract.md`. Kept as two files on purpose: a provider can be fully specified as data (in `Draft`) before any adapter code exists for it, and an adapter's code contract doesn't change based on which specific provider it's wired to. Conflating them would mean "is this provider registered" and "is this provider's code correct" are answered by the same document, when they're genuinely different questions with different owners.

## Shape

`../schemas/provider.schema.json` — the complete field contract. Required at minimum: `id`, `display_name`, `tier`, `status`, `deployment_type`, `auth`, `adapter_ref`, `schema_version`, `sources`. `adapter_ref` is the field that makes "never hardcode providers" real — it's a string the Registry resolves at load time, never a class imported by name anywhere in routing code.

## Lifecycle

```
draft → validating → published → deprecated → archived
                ↓
            rejected → (resubmit) → draft
```

`Validating` runs four checks, all four required, none optional:

1. Schema validation against `provider.schema.json`.
2. `id` uniqueness across the registry.
3. `adapter_ref` resolves to a real class satisfying `adapter-contract.md`'s `BaseProviderAdapter` shape.
4. One live `health_check()` call — a smoke test, not a synthetic assumption that the adapter works.

A provider that fails any of the four is `rejected`, not silently left in `draft` — rejection is a recorded, reasoned outcome (`VersionHistoryEntry`, `change_type=rejected`), not a dead end with no trail.

## Trust

`trust_level` (`first_party | verified_third_party | unverified`) is required on the owning `PluginManifest`, never defaulted. An `unverified` provider cannot reach `published` on technical checks alone — it additionally requires `registry_admin` sign-off and, for its capability claims specifically, a synthetic verification probe reusing the benchmark platform's own scoring machinery rather than a second bespoke verification system. Trust level and RBAC role are independent gates; neither alone is sufficient.

## Versioning

Every change to a published provider — a pricing update, a capability change, a status transition — appends a `VersionHistoryEntry` rather than overwriting the row in place. `changed_via=manual` requires `changed_by`; `change_type=deprecated` requires `reason`. The full history is queryable per provider; the current row is always the latest state, never the only state.
