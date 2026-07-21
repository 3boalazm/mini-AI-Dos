# Gateway Contract

The only layer external callers see. A client pointed at `/v1/chat/completions` never knows it's talking to hundreds of providers instead of one — that invisibility is the contract, not an implementation detail.

## Responsibilities, in this order, none skippable

1. Authenticate the caller against an AI-DOS API key — never a provider's own key. See `../api/authentication.md`.
2. Translate the OpenAI-shaped request body into the internal `UniversalRequest` family (`ChatRequest`, `EmbeddingRequest`, etc. — see `../sdk/typescript.md` for the canonical shape names).
3. Hand off to the Routing Engine. **The Gateway makes no routing decisions itself** — no provider selection, no failover logic, no cost estimation lives here. A Gateway that started making these decisions would duplicate logic that already has one owner.
4. Translate the internal response (or stream of chunks) back into OpenAI's JSON or SSE shape.
5. Map every typed provider error to the HTTP status and OpenAI-compatible error body a client SDK already knows how to handle — see `validation-rules.md` for the full mapping.

## Exposure boundary

Public: `/v1/*` completion routes, `/v1/providers`, `/v1/models` (read-only catalog views). **Never public**: `/internal/v1/*` (Registry writes, Discovery queries) — the Dashboard backend calls these internally and exposes only its own read-only views to browsers, never a direct proxy. See `../database/relationships.md` for the same boundary enforced at the database-role level, not only at the network layer.

## What the Gateway is not

Not a cache (that's the Cache Layer, consulted, not embedded). Not a rate limiter beyond its own inbound protection (outbound provider limits are the Rate Limit Manager's). Not a retry engine (tier-1 transport retry lives inside each adapter; tier-2 failover lives in the Routing Engine). The Gateway is thin by contract, not by omission — every one of these concerns has an owner elsewhere, and the Gateway's job is to not reach for them.
