# Relationships, Normalization, Partitioning, Sharding, Multi-Tenancy

## Relationships and cardinality

- **Organization 1—N Team, 1—N Project, 1—N User** — standard tenancy tree; **Project 1—N APIKey**.
- **Provider 1—N Model 1—N ModelVersion / Capability / ProviderPricing** — one-to-many throughout.
- **Model N—N ProviderRegion** via `ProviderRegionMapping` — genuinely many-to-many; a region hosts many providers, a provider spans many regions.
- **Organization 1—1 Subscription**, **Organization 1—N Invoice**.
- **RoutingPolicy 1—N RoutingRule**; **Project N—1 RoutingPolicy** (a project's default; a request can still override per-call).

## Normalization decisions

- **`MediaAsset` is one table for four modalities, not four.** Embedding/Image/Audio/Video rows are identical in every column except `modality` and what `object_storage_ref` points at. Four tables would mean four copies of the same schema and four places to enforce the same constraints, for no query benefit — nothing joins "images" specifically that wouldn't also want "media of any modality for this request."
- **`RequestLog` is not the parent of `ToolCall`/`StreamingSession`/`MediaAsset`.** All four key off `request_id` independently rather than nesting — at this volume, a request's full detail is assembled by querying each independently when needed (a specific debugging session), never by a routine join across all four for every request.
- **Business version history is never a column.** Every entity's *current* state is one row; every *change* to it is a separate, append-only `VersionHistoryEntry` — normalizing "what changed and why" out of the entity itself is what keeps an entity table narrow and a change history actually queryable as history, not reconstructed from diffing snapshots.

## Partitioning

| Table | Strategy | Why |
|---|---|---|
| HealthSignal | Daily range on `collected_at` | Highest-volume table in the system by a wide margin |
| BenchmarkResult | Monthly range on `measured_at` | Much lower volume, coarser partitioning is enough |
| RequestLog | Daily range on `occurred_at` | Sampled, but still high volume at this request count |
| UsageRecord | Monthly range on `occurred_at` | Aligns with billing period boundaries — invoice generation becomes a partition scan |
| EventLog | Weekly range on `published_at` | Lower volume than the above |

Retention is enforced by dropping old partitions, not `DELETE` — near-instant versus a row-by-row delete at this scale. HealthSignal: 30 days raw, then rolled up (see `../contracts/validation-rules.md` for what "rolled up" validates against) and the raw partition dropped. BenchmarkResult and VersionHistoryEntry: retained indefinitely — training data for regression detection and the audit trail, respectively, neither on a pruning timer.

## Sharding considerations

Not implemented now — Postgres with read replicas covers the stated scale for the write path, since writes concentrate in a small number of high-tenancy tables. **Shard key chosen in advance rather than left open**: `organization_id` — the natural multi-tenant boundary, and already the Row-Level Security partition key below. If sharding is ever needed, the RLS boundary and the shard boundary are the same boundary, not two decisions made independently later.

## Multi-tenant strategy

**Postgres Row-Level Security**, not application-layer filtering alone. Every tenant-scoped table carries `organization_id`; an RLS policy restricts every query to rows matching `current_setting('app.current_org_id')`, set once per request by the Go services from the authenticated caller's resolved identity, before any query runs. A bug in application-layer filtering logic can't leak cross-tenant data as a result — the database refuses the row regardless of what the application code did or didn't check.
