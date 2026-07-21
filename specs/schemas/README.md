# schemas/

Thirteen canonical, verified JSON Schemas — every one checked against the draft 2020-12 meta-schema, with unique `$id`s (see `_verify_all.py`).

| File | Entity |
|---|---|
| `provider.schema.json` | Provider |
| `model.schema.json` | Model |
| `benchmark_definition.schema.json`, `benchmark_result.schema.json` | Benchmark, BenchmarkResult |
| `health_signal.schema.json`, `health_score.schema.json` | ProviderHealth |
| `version_history_entry.schema.json` | ProviderVersion / AuditLog |
| `extension_point.schema.json`, `plugin_manifest.schema.json` | Plugin architecture |
| `organization.schema.json`, `project.schema.json`, `api_key.schema.json`, `routing_policy.schema.json` | Identity, tenancy, routing |

**Not yet split into standalone files here**: Pricing, Capability, ProviderRegion, ProviderLimits, ModelVersion, BenchmarkRun, RoutingRule, RoutingHistory, Team, User, UsageRecord, Invoice, Subscription, RequestLog, StreamingSession, ToolCall, PromptTemplate, PromptCache, MediaAsset, AgentSession, AgentMemory, Notification, FeatureFlag, EventLog, Job.

Every one of those is fully specified at the field level in `../database/entities.md` — types, nullability, defaults, constraints, indexes. Turning a field table into one of these files is a mechanical, already-demonstrated pattern (see any file above), not open design work. Run `_verify_all.py` after adding one.
