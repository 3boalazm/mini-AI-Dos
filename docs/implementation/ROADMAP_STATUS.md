# AI-DOS Roadmap Status

Authoritative, must reflect reality — never mark a phase done here without a corresponding `PHASE_<N>_REPORT.md` whose Definition of Done checklist is fully satisfied. See `EXECUTION_STATE.md` for the live in-progress detail and `ARCHITECTURE.md` for the as-built system shape.

## Roadmap revision history

**Revision 2 (current)** — a 12-phase plan (0–11) scoped to what `specs/` actually defines: a **Provider Gateway** (auth/API keys, gateway, provider adapters, usage/cost/metrics, org/project/RBAC, SDKs, reliability, hardening, integration/E2E, final readiness). No agent runtime, tool runtime, memory, orchestration/subagents, or channels/voice — those were Revision 1's scope and are no longer part of this roadmap. This matches `specs/database/entities.md`, `specs/api/openapi.yaml`, and `specs/contracts/*.md` far more directly than Revision 1 did; none of those files define an agent, a tool, or a memory entity.

**Revision 1 (superseded)** — the original 10-phase plan (Baseline → Database → Provider Platform → Identity → Gateway → **Agent Runtime → Secure Tool Runtime → Memory & Context → Orchestration & Subagents → Channels & Voice** → Production). Superseded before any phase past 0/1 was started — no work was lost, since Phases 2+ under Revision 1 never began. Phase 0 and Phase 1 carry over unchanged (same scope, renamed/renumbered consistently).

Note: a separate, unrelated Python codebase calling itself "AI-DOS — AI Delivery OS" was found on the `main` branch of `github.com/3boalazm/ai-dos` — a prior, self-admittedly mock-intelligence-only implementation of an agent-OS vision close to Revision 1's scope (see its own `docs/HONEST-REVIEW.md`, preserved at `legacy-before-ai-dos`). It is explicitly **not** part of this roadmap or this repository's implementation and is not evidence that any phase below is complete.

## Phase table

| Phase | Name                          | Status         | Report                                                             |
| ----- | ----------------------------- | -------------- | ------------------------------------------------------------------ |
| 0     | Foundation Verification       | 🟢 Complete    | [PHASE_0_REPORT.md](PHASE_0_REPORT.md), commit `9251a7a`           |
| 1     | Database Foundation           | 🔴 Blocked     | Docker Desktop installed but needs WSL2 — see `EXECUTION_STATE.md` |
| 2     | Authentication & API Keys     | ⬜ Not started | —                                                                  |
| 3     | Gateway                       | ⬜ Not started | —                                                                  |
| 4     | Provider Adapter System       | ⬜ Not started | —                                                                  |
| 5     | Usage, Cost & Metrics         | ⬜ Not started | —                                                                  |
| 6     | Organization / Project / RBAC | ⬜ Not started | —                                                                  |
| 7     | SDKs                          | ⬜ Not started | —                                                                  |
| 8     | Reliability / Observability   | ⬜ Not started | —                                                                  |
| 9     | Production Hardening          | ⬜ Not started | —                                                                  |
| 10    | Integration / E2E             | ⬜ Not started | —                                                                  |
| 11    | Final Production Readiness    | ⬜ Not started | —                                                                  |

Legend: ⬜ not started · 🟡 in progress · 🟢 complete (acceptance gate passed) · 🔴 blocked

## Known environment constraints affecting future phases

- **Docker Desktop installed but non-functional — needs WSL2, needs a human at the machine.** `winget install Docker.DockerDesktop` succeeded, but Docker Desktop has no backend to run on: this machine has never had WSL enabled, and enabling it (`wsl --install`) requires an elevated/interactive session no automated shell can provide. Blocks Phase 1 until the user runs `wsl --install` from an Administrator shell and restarts Windows.
- **No Python on the dev machine** — only blocks re-running `specs/schemas/_verify_all.py`; does not block any phase's implementation work.
- **No C compiler** — blocks Go's `-race` detector specifically. Does not block normal `go build`/`go test`/`go vet`.
- **Phase 2 (Authentication) will need OIDC/SSO provider decisions and credentials**; **Phase 4 (Provider Adapters) will need real LLM provider API keys** for anything beyond contract-shape tests. These will be raised as explicit blockers when those phases start, not assumed or fabricated.

## Remote repository state (for context on future sessions)

`github.com/3boalazm/ai-dos`:

- `main` (default branch) — this repository's history, currently at commit `d0aa4f4` (matches local at time of writing).
- `foundation` — same content as `main` as of the replacement; an artifact of the push sequence, left in place, not required for anything going forward.
- `legacy-before-ai-dos` — the prior unrelated Python codebase that used to occupy `main`, preserved exactly, untouched, not merged. See "Roadmap revision history" above.
