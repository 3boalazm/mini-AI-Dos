# Provider Stack Decision — Free-First, Multi-Provider

Status: **approved working decision** (2026-08-09). This documents which upstream LLM providers AI-DOS builds on and why. It pre-answers the blocker recorded in [ROADMAP_STATUS.md](ROADMAP_STATUS.md) ("Phase 5 (Provider Adapters) will need real LLM provider API keys"). Implementation of multi-provider routing remains Phase 4/5 scope — nothing here changes phase ordering.

## Decision

Free-first + high-limit + automatic failover, instead of building on paid Claude/GPT APIs. All selected providers expose OpenAI-compatible endpoints, so the existing single adapter (`services/gateway/internal/provider/openai.go`, configurable `BaseURL`) covers every tier — adding a provider is registry/config work, not a new implementation.

## Fallback chain

| Tier | Provider                                                                   | Role                                                                                    | Endpoint                                                  |
| ---- | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| 1    | **Google Gemini** (3.6 Flash primary; 3.5 Flash / 3.5 Flash-Lite for bulk) | Primary                                                                                 | `https://generativelanguage.googleapis.com/v1beta/openai` |
| 2    | **Groq**                                                                   | Fast worker / coding / first fallback                                                   | `https://api.groq.com/openai/v1`                          |
| 3    | **OpenRouter `openrouter/free`**                                           | Emergency router only — selects a free model at random per request; never primary       | `https://openrouter.ai/api/v1`                            |
| 4    | **Local Qwen3.6 via Ollama/vLLM**                                          | Hardware-bound last resort, no API quota                                                | local                                                     |
| —    | **Mistral**                                                                | OCR/documents lane only (PDF/scan/tables → structured data), never general chat routing | `https://api.mistral.ai/v1`                               |

## Verified facts (2026-08-09, live checks)

- Gemini key valid (HTTP 200); `gemini-3.6-flash` present on the key's model list; gateway smoke-tested end-to-end against it (real completion returned through `POST /v1/chat/completions`).
- Groq key valid; catalog on this key includes `openai/gpt-oss-120b`, `qwen/qwen3.6-27b`, `llama-3.3-70b-versatile`, `whisper-large-v3`, and Arabic TTS `canopylabs/orpheus-arabic-saudi`.
- OpenRouter key valid, free tier. `openrouter/free` router launched Feb 2026.
- Mistral key valid.
- Qwen3.6-27B (2026-04-22) and Qwen3.6-35B-A3B (2026-04-16) are open-weight, Apache 2.0 — the local-tier models.

## Tier-2 additions (no-card, renewable free tiers — 2026-08-09)

Found while restoring cloud access to DeepSeek/Qwen (absent from OpenRouter's `:free` rotation that day):

- **NVIDIA NIM** (`https://integrate.api.nvidia.com/v1`) — **verified working**: live inference tested on `deepseek-ai/deepseek-v4-flash-0731`. Key is 6-month validity (expires 2027-02-08 — renew at build.nvidia.com). Catalog that day: 100 models incl. `moonshotai/kimi-k2.6`, `z-ai/glm-5.2`; **no Qwen on its model list** despite third-party claims — Qwen coverage stays with Groq. ~40 RPM practical limit.
- **Cloudflare Workers AI** — **verified working**: live inference tested on `@cf/openai/gpt-oss-20b` via the account-scoped OpenAI-compat endpoint (`/accounts/<id>/ai/v1`). 10,000 neurons/day resetting daily 00:00 UTC; neuron pool is shared across models. Catalog highlights: `gpt-oss-120b`, `qwen2.5-coder-32b-instruct`, `qwen3-30b-a3b`, `deepseek-r1-distill-qwen-32b`, `whisper`, FLUX 2 image gen, `qwen3-embedding`. Gotcha recorded: the account ID is the 32-hex string in the dashboard URL — a dashed UUID from elsewhere in the UI is a different identifier and yields 403 "Authentication error".
- **SambaNova Cloud** (`https://api.sambanova.ai/v1`) — **verified working**: live inference tested on `DeepSeek-V3.1`. Forever-free developer tier, no card; per-model daily caps vary. Catalog that day: `DeepSeek-V3.1`, `DeepSeek-V3.2` (full model, not a distill — the only free full-DeepSeek in the stack), `MiniMax-M2.7`, `Meta-Llama-3.3-70B-Instruct`, `gemma-4-31B-it`, `gpt-oss-120b`.

Excluded from candidates: **DeepSeek direct API** — 5M-token one-time grant (30 days), then a payment method is mandatory; fails the "renewable without card" rule. DeepSeek access comes via NVIDIA NIM / Cloudflare instead.

## Local tier status (2026-08-09)

Ollama v0.22.1 installed at `E:\ollama\app` (installer signature verified: Ollama Inc.), models at `E:\ollama\models` (`OLLAMA_MODELS` user env var — C: had no space). Live now: **`nomic-embed-text`** (274MB, local embeddings, inference-verified). **`qwen3.5:4b` postponed by the user**: its download repeatedly wedged at the same byte (~93%) across 4 resume attempts including a full server restart — the partial blob was corrupt and has been deleted so a future `ollama pull qwen3.5:4b` starts clean (~3.4GB fresh). A small-model download completed at full speed, proving the network path is fine.

## Excluded, with evidence

- **GitHub Models** — was in the original plan; **fully retired 2026-07-30** (its API returned HTTP 410 `github_models_retirement_brownout` on 2026-08-09; confirmed by GitHub's changelog). Permanently dropped.
- **Cerebras** — signup requires an international credit card the project owner doesn't have. No capability loss: its headline model (gpt-oss-120b) is served by Groq. Its free tier also carried an ~8K context cap, which limited its fallback value anyway. Revisit only if payment options change.

## Operating rules

1. **Never hardcode upstream model IDs** in gateway code. Model selection resolves through a Model Registry at runtime (`specs/api/models.md` direction). `AI_MODEL` in `.env` is a development default, not an architectural exception.
2. **Re-verify free-tier rate limits live at integration time** — third-party blog numbers are stale/contradictory; only provider dashboards and official rate-limit docs count.
3. **Keys live in the repo-root `.env` only** (gitignored; `.env.example` documents shape without values). The vault section there holds per-provider keys that Phase 4/5 will consume.
4. **Thinking-model caveat**: `gemini-3.6-flash` spends reasoning tokens from `max_tokens`; small values yield an empty response with `finish_reason:"length"` and `completion_tokens:0`. Routing/defaults must account for reasoning-token overhead on thinking models.
