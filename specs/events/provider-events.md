# Provider Events

Conventions shared by every event file in this directory: envelope `{event_id, topic, occurred_at, payload}`; `event_id` is the idempotency key, always; ordering guarantees exist only where stated, never assumed.

| Event | Producer | Consumers | Payload | Ordering | Retry / DLQ |
|---|---|---|---|---|---|
| `ProviderRegistered` | Registry | Discovery, Dashboard | `Provider` (draft state) | Per `provider_id` subject | Aggressive |
| `ProviderUpdated` | Registry | Discovery, Dashboard | `Provider` + `changed_fields[]` | Per `provider_id` subject | Aggressive |
| `ProviderUnavailable` | Health Scorer | Selection Algorithm, Alerting | `HealthScore` (circuit_state=open) | Per `provider_id` subject | Aggressive; naturally idempotent |
| `HealthChanged` | Health Scorer | Selection Algorithm, Dashboard | `HealthScore` | Per `provider_id` subject | Aggressive |

`ProviderUnavailable` and `HealthChanged` are the production names for what the phase-2 architecture originally called `health.degraded`/`health.recovered` — renamed once the full circuit-breaker state machine existed to name the actual transition, not the informal description of it.
