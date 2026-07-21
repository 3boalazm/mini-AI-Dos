# Routing Events

| Event | Producer | Consumers | Payload | Ordering | Retry / DLQ |
|---|---|---|---|---|---|
| `RoutingDecisionCreated` | Routing Engine | Analytics, `RoutingHistory` writer | `RoutingHistory` row shape | None required | Moderate; sampled, not every decision, at production volume |
| `RateLimitExceeded` | Rate Limit Manager | Selection Algorithm, Alerting | `{provider_id, retry_after_seconds}` | None | Aggressive; naturally idempotent |
| `CostThresholdReached` | Cost Optimization Engine | Priority Engine, Alerting | `{organization_id, threshold, current_spend}` | Per `organization_id` | Aggressive + immediate DLQ alert — financial consequence, not just data quality |
