# Request Events

| Event | Producer | Consumers | Payload | Ordering | Retry / DLQ |
|---|---|---|---|---|---|
| `RequestStarted` | Gateway | Tracing/metrics pipeline only | `{request_id, trace_id, model_id}` | None | Best-effort, no DLQ — a metrics signal, losing one is not a correctness issue |
| `RequestCompleted` | Gateway | Usage Service, Analytics | `{request_id, status_code, latency_ms, usage: {input_tokens, output_tokens}}` | None | Aggressive — Usage Service billing depends on this |
| `StreamingStarted` / `StreamingCompleted` | Gateway | `StreamingSession` writer | `{request_id, chunks_sent}` on completion | Ordered per `request_id` | Moderate |

**`RequestCompleted` is the one event in this entire directory where a dedup failure has direct financial consequence, not just a data-quality one.** A consumer implementer who treats every row across all five event files as uniformly "nice to dedup" rather than "this specific one is billing-critical" is exactly the kind of gap that doesn't surface until an invoice is wrong — flagged here explicitly rather than left to read the same as `RequestStarted` two rows up.
