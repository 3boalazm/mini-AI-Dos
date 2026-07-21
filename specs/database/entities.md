# Entities

Every entity in the system, field by field. Nine already ship as standalone JSON Schemas in `../schemas/` (listed there, not repeated here in full — this file gives their purpose and key relationships only). The remaining ~28 get their complete field-level spec here.

## Standard conventions (apply to every entity, stated once)

- **Primary key**: `id`, UUIDv7.
- **Audit fields**: `created_at`, `updated_at` (`timestamptz`, not null) on every table.
- **Soft delete**: `deleted_at` (nullable) on business entities where recoverable deletion matters — Organization, Project, User, APIKey, Provider, Model, PromptTemplate, Subscription. Hard delete elsewhere, governed by retention + partition drop.
- **Versioning**: `schema_version` where row shape can change independently of data. Business version *history* is never a column on the row — it's a `VersionHistoryEntry`, for every entity here, not only Provider/Model.
- **Multi-tenant column**: `organization_id` on every tenant-scoped table — everything except the Provider/Model/Benchmark/Health/Plugin cluster, which is platform-global.

## Already-specified entities (see `../schemas/`)

| Entity | Purpose |
|---|---|
| Provider | A registered AI provider — hosted API, router, or self-hosted endpoint |
| Model | One model offered by one provider |
| Benchmark / BenchmarkResult | What gets measured, and one measurement |
| ProviderHealth (HealthSignal / HealthScore) | Raw observations and scored status/circuit state |
| ProviderVersion (VersionHistoryEntry) | Append-only audit trail for Provider and Model changes |
| ExtensionPoint / PluginManifest | The plugin system's own catalog |
| Organization / Project / APIKey / RoutingPolicy | Tenancy root, workspace, credential metadata, named routing strategy |

## Provider domain — completing what was sketched

### ProviderPricing
| Field | Type | Null | Default | Notes |
|---|---|---|---|---|
| model_id | uuid FK→Model | no | — | |
| unit | enum | no | — | per_1m_input_tokens \| per_1m_output_tokens \| per_1m_cached_tokens \| per_request \| per_second |
| price | numeric(12,6) | no | — | |
| currency | char(3) | no | 'USD' | |
| effective_from | date | no | — | |
| effective_to | date | yes | null | null = currently in effect |
| source | text | no | — | |

Indexes: composite `(model_id, unit, effective_from)`; partial `WHERE effective_to IS NULL`. Constraint: `CHECK (effective_to IS NULL OR effective_to > effective_from)`.

### ProviderLimits
| Field | Type | Null | Default |
|---|---|---|---|
| provider_id | uuid FK→Provider | no | — |
| requests_per_minute | integer | yes | null |
| tokens_per_day | bigint | yes | null |
| concurrent_requests | integer | yes | null |
| notes | text | yes | null |

Unique index: `(provider_id)`.

### ProviderRegion / ProviderRegionMapping
| Field | Type | Null |
|---|---|---|
| id (Region) | uuid | no |
| code | text | no |
| data_residency_notes | text | yes |
| provider_id (mapping) | uuid FK→Provider | no |
| region_id (mapping) | uuid FK→ProviderRegion | no |

Mapping composite PK: `(provider_id, region_id)` — genuinely many-to-many.

## Model domain

### Capability
| Field | Type | Null | Default |
|---|---|---|---|
| model_id | uuid FK→Model | no | — |
| tool_calling | enum | no | 'none' |
| mcp_support | enum | no | 'none' |
| vision, reasoning, embeddings, rerank, image_gen, structured_output, streaming | boolean | no | false |
| computer_use | enum | no | 'none' |
| max_context_window_tokens | integer | yes | null |
| extensions | jsonb | no | '{}' |

Unique index: `(model_id)`.

### ModelVersion
| Field | Type | Null | Default | Notes |
|---|---|---|---|---|
| model_id | uuid FK→Model | no | — | canonical/current row |
| version_tag | text | no | — | provider's dated release name |
| released_at | date | yes | null | |
| superseded_by | uuid FK→ModelVersion | yes | null | self-referential release chain |
| capability_snapshot_ref | uuid FK→Capability | yes | null | |

Index: `(model_id, released_at DESC)`.

## Benchmark domain

### BenchmarkRun
| Field | Type | Null | Default |
|---|---|---|---|
| benchmark_id | uuid FK→Benchmark | no | — |
| model_id | uuid FK→Model | no | — |
| status | enum | no | 'queued' |
| triggered_by | enum | no | — |
| started_at, completed_at | timestamptz | yes | null |
| items_total, items_completed | integer | yes/no | null / 0 |

Index: `(benchmark_id, model_id, started_at DESC)`.

## Routing domain

### RoutingPolicy — see `../schemas/routing_policy.schema.json`.

### RoutingRule
| Field | Type | Null |
|---|---|---|
| routing_policy_id | uuid FK→RoutingPolicy | no |
| condition | jsonb | no |
| weight_adjustment | numeric(5,4) | yes |
| priority | integer | no |

Index: `(routing_policy_id, priority)`.

### RoutingHistory
| Field | Type | Null |
|---|---|---|
| request_id | uuid | no |
| organization_id | uuid FK | no |
| candidates_considered | jsonb | no |
| winning_model_id | uuid FK→Model | no |
| decided_at | timestamptz | no |

## Identity and tenancy domain

### Team
| Field | Type | Null |
|---|---|---|
| organization_id | uuid FK | no |
| name | text | no |

### User
| Field | Type | Null | Default |
|---|---|---|---|
| organization_id | uuid FK | no | — |
| email | citext | no | — |
| identity_provider_subject | text | no | — |
| role | enum | no | 'member' |

Unique index: `(email)`.

## Usage and billing domain

### UsageRecord
| Field | Type | Null |
|---|---|---|
| organization_id | uuid FK | no |
| project_id | uuid FK→Project | no |
| model_id | uuid FK→Model | no |
| input_tokens, output_tokens | bigint | no |
| computed_cost | numeric(12,6) | no |
| occurred_at | timestamptz | no |

### Invoice
| Field | Type | Null |
|---|---|---|
| organization_id | uuid FK | no |
| period_start, period_end | date | no |
| total_amount | numeric(12,2) | no |
| status | enum | no |
| stripe_invoice_id | text | yes |

### Subscription
| Field | Type | Null |
|---|---|---|
| organization_id | uuid FK | no |
| plan | enum | no |
| stripe_subscription_id | text | yes |
| current_period_end | timestamptz | no |

## Request domain

**Design decision**: not a row-per-request table at "millions of requests" — that volume belongs in traces and metrics. These tables are for sampled retention and structured facets only.

### RequestLog
| Field | Type | Null |
|---|---|---|
| request_id | uuid | no |
| organization_id | uuid FK | no |
| model_id | uuid FK→Model | no |
| status_code | integer | no |
| latency_ms | integer | no |
| sampled_content_ref | text | yes |

### StreamingSession
| Field | Type | Null |
|---|---|---|
| request_id | uuid | no |
| chunks_sent | integer | no |
| started_at, completed_at | timestamptz | started not null, completed nullable |

### ToolCall
| Field | Type | Null |
|---|---|---|
| request_id | uuid | no |
| tool_name | text | no |
| arguments_valid | boolean | no |
| result_ref | text | yes |

## Content domain

### PromptTemplate
| Field | Type | Null | Default |
|---|---|---|---|
| project_id | uuid FK | no | — |
| name | text | no | — |
| template_body | text | no | — |
| variables | jsonb | no | '[]' |
| version | integer | no | 1 |

Unique index: `(project_id, name, version)`.

### PromptCache
| Field | Type | Null |
|---|---|---|
| prompt_hash | text | no |
| model_id | uuid FK | no |
| provider_cache_ref | text | yes |
| expires_at | timestamptz | no |

### MediaAsset
One table for embeddings/images/audio/video metadata, not four — see `relationships.md` for the normalization reasoning.

| Field | Type | Null |
|---|---|---|
| request_id | uuid | no |
| modality | enum | no |
| object_storage_ref | text | no |
| content_hash | text | no |

## Agent domain

### AgentSession
| Field | Type | Null |
|---|---|---|
| project_id | uuid FK | no |
| status | enum | no |
| started_at | timestamptz | no |

### AgentMemory
| Field | Type | Null |
|---|---|---|
| agent_session_id | uuid FK | no |
| memory_type | enum | no |
| content_ref | text | no |
| embedding_ref | uuid FK→MediaAsset | yes |

## Platform domain

### Notification
| Field | Type | Null |
|---|---|---|
| organization_id | uuid FK | no |
| user_id | uuid FK | yes |
| kind | text | no |
| payload | jsonb | no |
| read_at | timestamptz | yes |

### FeatureFlag
| Field | Type | Null | Default |
|---|---|---|---|
| key | text | no | — |
| enabled_globally | boolean | no | false |
| enabled_for_org_ids | uuid[] | no | '{}' |

### EventLog
| Field | Type | Null |
|---|---|---|
| event_id | uuid | no |
| topic | text | no |
| payload | jsonb | no |
| published_at | timestamptz | no |

### Job
| Field | Type | Null |
|---|---|---|
| request_id | uuid | no |
| job_type | text | no |
| status | enum | no |
| result_ref | text | yes |
