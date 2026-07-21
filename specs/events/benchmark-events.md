# Benchmark Events

| Event | Producer | Consumers | Payload | Ordering | Retry / DLQ |
|---|---|---|---|---|---|
| `BenchmarkCompleted` | Benchmark Runner | Registry, Dashboard | `BenchmarkResult` | None required — independent per run | Moderate |
| `AlertTriggered` | Alerting | PagerDuty/Opsgenie bridge | `{severity, source_event_id, summary}` | None | Aggressive; `event_id` dedup — a duplicated page is preferable to a missed one, but dedup still wins over spamming |

`AlertTriggered` sits here rather than a separate `alerting-events.md` because in practice its most frequent trigger, in this system, is a benchmark regression or drift detection — it's listed under the domain that fires it most, not the domain it structurally belongs to, since forcing a rigid one-event-one-file rule would fragment a single alerting story across several files for no reader benefit.
