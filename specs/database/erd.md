# Entity-Relationship Diagrams

Two diagrams, not one — the full system spans two clusters loosely connected through `Model`/`Organization`, and one combined diagram of all ~37 entities would be unreadable. Both render in any mermaid-compatible viewer.

## Core platform: Provider, Model, Benchmark, Health, Plugins

```mermaid
erDiagram
  MODEL }o--|| PROVIDER : "offered by"
  MODEL ||--o| PRICING_SCHEMA : "priced by"
  MODEL ||--o| CAPABILITY_SCHEMA : describes
  BENCHMARK_DEFINITION ||--|| BENCHMARK_DATASET : uses
  BENCHMARK_DEFINITION ||--o{ BENCHMARK_RUN : "executed as"
  BENCHMARK_RUN ||--o{ BENCHMARK_RESULT : produces
  BENCHMARK_RESULT }o--|| MODEL : measures
  HEALTH_SIGNAL }o--o{ HEALTH_SCORE : "contributes to"
  HEALTH_SIGNAL }o--|| PROVIDER : observes
  HEALTH_SCORE }o--|| PROVIDER : scores
  PROVIDER ||--o{ VERSION_HISTORY_ENTRY : logs
  MODEL ||--o{ VERSION_HISTORY_ENTRY : logs
  EXTENSION_POINT_DEFINITION ||--o{ PLUGIN_MANIFEST : "implemented by"

  PROVIDER { string id PK  string status }
  MODEL { string id PK  string provider_id FK  string status }
  PRICING_SCHEMA { string id PK  string model_id FK }
  CAPABILITY_SCHEMA { string id PK  string model_id FK }
  BENCHMARK_DEFINITION { string id PK  string category }
  BENCHMARK_DATASET { string id PK  string license_notes }
  BENCHMARK_RUN { string id PK  string benchmark_id FK }
  BENCHMARK_RESULT { string id PK  string model_id FK  string methodology }
  HEALTH_SIGNAL { string id PK  string source }
  HEALTH_SCORE { string id PK  string status  string circuit_state }
  VERSION_HISTORY_ENTRY { string id PK  string entity_type }
  EXTENSION_POINT_DEFINITION { string id PK  string cardinality }
  PLUGIN_MANIFEST { string plugin_id PK  string trust_level }
```

## Identity, tenancy, routing, billing

```mermaid
erDiagram
  ORGANIZATION ||--o{ TEAM : has
  ORGANIZATION ||--o{ USER : employs
  ORGANIZATION ||--o{ PROJECT : owns
  TEAM ||--o{ PROJECT : "may scope"
  PROJECT ||--o{ APIKEY : issues
  PROJECT ||--o| ROUTING_POLICY : "defaults to"
  ROUTING_POLICY ||--o{ ROUTING_RULE : contains
  PROJECT ||--o{ USAGE_RECORD : accrues
  ORGANIZATION ||--o{ INVOICE : billed
  ORGANIZATION ||--|| SUBSCRIPTION : has

  ORGANIZATION { string id PK  string tier }
  TEAM { string id PK  string organization_id FK }
  USER { string id PK  string organization_id FK  string role }
  PROJECT { string id PK  string organization_id FK  string team_id FK }
  APIKEY { string id PK  string project_id FK  string key_hash }
  ROUTING_POLICY { string id PK  string organization_id FK }
  ROUTING_RULE { string id PK  string routing_policy_id FK }
  USAGE_RECORD { string id PK  string project_id FK }
  INVOICE { string id PK  string organization_id FK  string status }
  SUBSCRIPTION { string id PK  string organization_id FK  string plan }
```

**The bridge between the two diagrams**: `UsageRecord.model_id → Model.id` and `RoutingHistory.winning_model_id → Model.id` (see `relationships.md`) — every tenant-scoped request ultimately resolves to a core-platform `Model`, which is why the two clusters are diagrammed separately but aren't actually independent.
