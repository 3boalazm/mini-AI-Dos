# Models API

| Route | Method | Auth | Notes |
|---|---|---|---|
| `/v1/models` | GET | API key | Matches OpenAI-compatible `GET /v1/models` wire shape; filtered to the caller's entitlements, never a static list |
| `/v1/models/{id}/compatible` | GET | API key | Computed on demand against live capability indexes — never a stored pairwise table, see `../database/relationships.md` |
| `/v1/models/{id}/pricing` | GET | API key | Full `ProviderPricing` time series, not just the current price — historical cost auditing needs the whole series |
| `/v1/benchmarks` | GET | API key | `Benchmark` catalog |
| `/v1/benchmarks/{id}/results` | GET | API key | `BenchmarkResult`, `methodology`-tagged — vendor claim vs. AI-DOS-measured are never conflated in the response |

Full field contract: `../schemas/model.schema.json`, `../schemas/benchmark_result.schema.json`. `Capability`, `ModelVersion`, `BenchmarkRun`: `../database/entities.md`.
