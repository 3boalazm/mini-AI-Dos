# Adapter Contract

The **code** contract. Every provider — frontier API, router, or self-hosted endpoint — implements exactly this shape, regardless of what the underlying provider's own API looks like. See `provider-contract.md` for the data contract this code is registered against.

## Base contract — every adapter, no exceptions

```
BaseProviderAdapter
  provider_id: string
  lifecycle_state() -> uninitialized | initializing | ready | degraded | shutting_down | shutdown
  initialize(context: PluginContext) -> void
  shutdown() -> void
  health() -> HealthStatus
  capabilities() -> CapabilitySchema
  estimateCost(request) -> CostEstimate
  estimateLatency(request) -> LatencyEstimate
  tokenize(text: string, modelId: string) -> TokenizeResult
```

`tokenize()` and `estimateLatency()` are new at the base level (this phase) — every adapter must expose its own tokenizer rather than assume a universal one (OpenAI's, Anthropic's, and Google's token counts for identical text all differ), and cost/latency are now both pre-dispatch signals a candidate can be filtered on, not cost alone.

## Modality interfaces — implement only what the provider actually supports

| Interface | Method | 
|---|---|
| `ChatAdapter` | `chat(request) -> ChatResponse` |
| `EmbeddingAdapter` | `embeddings(request) -> EmbeddingResponse` |
| `ImageAdapter` | `image(request) -> ImageResponse` |
| `AudioAdapter` | `audio.transcribe(request)`, `audio.synthesize(request)` |
| `VideoAdapter` | `video(request) -> Job` (never synchronous) |

## Optional capability mixins — composable, not modality-bound

| Mixin | Method | Composes with |
|---|---|---|
| `StreamingCapable` | `stream(request) -> Stream<Chunk>` | Any modality — chat tokens, TTS audio, video progress all stream the same way |
| `AsyncJobCapable` | `submitJob`, `getJobStatus`, `getJobResult` | `VideoAdapter` always; optionally `ImageAdapter`, `ChatAdapter` (batch) |
| `RerankCapable` | `rerank(request) -> RerankResponse` | New this phase — same interface-segregation shape as the others |
| `BatchCapable` | `batch(requests) -> BatchJob` | New this phase — composes with `AsyncJobCapable`, the same submit-then-poll shape `VideoAdapter` already uses, applied to chat instead |

Streaming is a mixin, not a sixth modality interface, because it isn't a modality — it's a delivery mode that cuts across several. Forcing every modality through its own `StreamX` interface would mean five near-identical shapes differing only in payload type.

## Error taxonomy — the shared vocabulary every method raises into

`ProviderError` (base) → `RateLimitedError`, `AuthError`, `ContextLengthExceededError`, `ContentPolicyError`, `ProviderUnavailableError`, `UnsupportedCapabilityError`, `InvalidRequestError`, `PayloadTooLargeError`, `ProviderTimeoutError`. Never a bare exception — the Failover System classifies by type, not by inspecting a message string.

## Lifecycle

`uninitialized → initializing → ready ⇄ degraded → shutting_down → shutdown` — the adapter's own self-report, kept explicitly separate from `circuit_state` (`closed | open | half_open`), which is Health Monitoring's external judgment based on aggregate behavior. A `ready` adapter can sit behind an `open` circuit; the two answer different questions.

## Retry — two tiers, not one

- **Tier 1, inside the adapter, invisible above it**: pure transport failures (connection reset, DNS, first-attempt timeout) — 2–3 retries, exponential backoff, never surfaces as a typed error.
- **Tier 2, the Failover System, once tier 1 gives up**: the adapter raises and stops. It classifies; it never decides whether to retry.

## Capability negotiation

Detection is one-time, at load — self-reported `capabilities()` diffed against the Registry's catalog entry, mismatch logged both directions. Negotiation is per-request — a specific request's needs checked against the winning candidate's declared capabilities, with three outcomes: fully supported (dispatch), partially supported (degrade explicitly, never silently drop a parameter), unsupported (`UnsupportedCapabilityError`, try the next candidate).
