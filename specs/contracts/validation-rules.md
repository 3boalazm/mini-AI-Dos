# Validation Rules

One technique — JSON Schema, `additionalProperties: false`, conditional `if/then` for cross-field rules — applied at every surface below. A new surface added later doesn't need a new validation strategy designed for it; it needs this one applied.

| Surface | Validated by | Where |
|---|---|---|
| API requests | JSON Schema, typed fields | Gateway boundary, before Routing — lenient toward unrecognized fields from external clients, strict for internal callers |
| Provider responses | Adapter's translation output re-validated against the Universal* response shape | Adapter Layer, before returning from any modality method |
| Streaming | Each chunk validated independently | Adapter Layer, per chunk — one malformed chunk is dropped and logged, not treated as invalidating the whole stream |
| Tool calling | Arguments validated against the tool's own declared schema | Gateway, before dispatching a tool call |
| JSON mode / structured outputs | Model output validated against the caller-supplied target schema | Gateway, post-response, before returning to the caller |
| Events | Producer validates before publish; consumer re-validates on receipt | Event Bus boundary, both sides |
| Database | `CHECK` constraints for row-local rules; application layer for anything spanning rows | Write path |

## Error-to-HTTP mapping (referenced by `gateway-contract.md`)

| `ProviderError` subclass | HTTP status | OpenAI-compatible error type |
|---|---|---|
| `RateLimitedError` | 429 | `rate_limit_exceeded` |
| `AuthError` | 401 | `invalid_api_key` |
| `InvalidRequestError` | 400 | `invalid_request_error` |
| `ContextLengthExceededError` | 400 | `context_length_exceeded` |
| `ContentPolicyError` | 400 | `content_policy_violation` |
| `ProviderUnavailableError` | 503 | `provider_unavailable` |
| `UnsupportedCapabilityError` | 400 | `unsupported_capability` |
| `PayloadTooLargeError` | 413 | `payload_too_large` |
| `ProviderTimeoutError` | 504 | `timeout` |

## Deprecation annotation

A field slated for removal carries `"deprecated": true` plus a `description` naming the replacement and a target removal version — an annotation, not a validation-relaxing keyword. A deprecated field stays exactly as required or optional as it was; deprecation announces intent, it doesn't itself loosen the contract.

## The one rule underneath all of this

Every conditional (`circuit_state=open → reason required`, `methodology=aidos_internal → confidence required`, `trust_level` required with no default, and every other `if/then` in `../schemas/`) has been proven, not just written — checked against a real instance built specifically to violate it, confirmed to fail for the stated reason. A rule that's never been tested against its own violation is a comment, not a constraint.
