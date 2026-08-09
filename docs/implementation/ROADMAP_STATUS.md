# AI-DOS Roadmap Status

Authoritative, must reflect reality — never mark a phase done here without a corresponding `PHASE_<N>_REPORT.md` whose Definition of Done checklist is fully satisfied. See `EXECUTION_STATE.md` for the live in-progress detail and `ARCHITECTURE.md` for the as-built system shape.

## Roadmap revision history

**Revision 4 (current) — agent expansion track added alongside, nothing renumbered.** The user approved (2026-08-09) reinstating agent scope — dropped in Revision 2 because `specs/` did not define it — as an explicit product decision: a phased single-agent track (core loop → tools → self-inspection → workspace → approvals → recovery → git), documented authoritatively in [AGENT_ROADMAP.md](AGENT_ROADMAP.md). The platform phases below are unchanged and keep their numbering; the agent track runs in parallel and reuses the gateway's provider seam. Specs for the agent surface are acknowledged debt owed by that track.

**Revision 3 (current, APPROVED as working baseline)** — inserts a new **Phase 2 — Organization & Project Foundation** (minimal `Organization`/`Project` schema + repository slice) between Database Foundation and Authentication, because `APIKey` (`specs/schemas/api_key.schema.json`) requires `project_id`, `specs/api/authentication.md` states API-key identity resolution _is_ `APIKey.project_id → Organization`, and `RoutingPolicy` (`specs/schemas/routing_policy.schema.json`) requires `organization_id` directly — Revision 2 had both Authentication (old Phase 2) and Gateway (old Phase 3) depending on Organization/Project data that old Phase 6 wasn't scheduled to create until after them. **`RoutingPolicy` is Organization-scoped directly, not via Project** (`routing_policy.schema.json` has no `project_id` field) — this correction is load-bearing and must stay explicit wherever this dependency is described. Full dependency graph, evidence citations, exact minimal scope, exclusions, and Definition of Done: [PHASE_2_SCOPE.md](PHASE_2_SCOPE.md) (approved in principle; implementation not yet started). Old Phase 6 (Organization/Project/RBAC) is renamed **Organization & RBAC (Advanced)** and renumbered to 7 — it retains everything not pulled into the new minimal Phase 2 (full org management, billing UX, quotas, advanced RBAC, invitations, `Team`).

**Phase 1 scope, separately determined and documented in [PHASE_1_SCOPE.md](PHASE_1_SCOPE.md)**: tooling and entity-agnostic convention only (connection pooling, `golang-migrate`, generic repository pattern, RLS/soft-delete/`schema_version` conventions, all integration-tested against real Postgres) — **zero domain entities**, including `Organization`/`Project`, which stay Phase 2's exclusively. Reached by evidence (`specs/database/migrations-strategy.md`/`erd.md` are entity-agnostic; the codebase's own `errors.go` doc comment states the "epic N shouldn't reach into epic M's scope" principle explicitly) after finding that the roadmap/TDS documents `docs/architecture-references.md` cites as authoritative for epic scoping (`00-roadmap-overview.md`, `01-sprint-1-detailed-tasks.md`, a "Technical Design Specification") **do not exist anywhere in this repository** — a dangling-reference gap, not resolved here.

**Revision 2 (superseded)** — a 12-phase plan (0–11) scoped to what `specs/` actually defines: a **Provider Gateway** (auth/API keys, gateway, provider adapters, usage/cost/metrics, org/project/RBAC, SDKs, reliability, hardening, integration/E2E, final readiness). No agent runtime, tool runtime, memory, orchestration/subagents, or channels/voice — those were Revision 1's scope and are no longer part of this roadmap. This matches `specs/database/entities.md`, `specs/api/openapi.yaml`, and `specs/contracts/*.md` far more directly than Revision 1 did; none of those files define an agent, a tool, or a memory entity. Superseded because Authentication/Gateway (its Phases 2/3) had an unsatisfiable dependency on Organization/Project data scheduled for its Phase 6 — see Revision 3 above.

**Revision 1 (superseded)** — the original 10-phase plan (Baseline → Database → Provider Platform → Identity → Gateway → **Agent Runtime → Secure Tool Runtime → Memory & Context → Orchestration & Subagents → Channels & Voice** → Production). Superseded before any phase past 0/1 was started — no work was lost, since Phases 2+ under Revision 1 never began. Phase 0 and Phase 1 carry over unchanged (same scope, renamed/renumbered consistently).

Note: a separate, unrelated Python codebase calling itself "AI-DOS — AI Delivery OS" was found on the `main` branch of `github.com/3boalazm/ai-dos` — a prior, self-admittedly mock-intelligence-only implementation of an agent-OS vision close to Revision 1's scope (see its own `docs/HONEST-REVIEW.md`, preserved at `legacy-before-ai-dos`). It is explicitly **not** part of this roadmap or this repository's implementation and is not evidence that any phase below is complete.

## Phase table

| Phase | Name                              | Status                   | Report                                                                                             |
| ----- | --------------------------------- | ------------------------ | -------------------------------------------------------------------------------------------------- |
| 0     | Foundation Verification           | 🟢 Complete              | [PHASE_0_REPORT.md](PHASE_0_REPORT.md), commit `9251a7a`                                           |
| 1     | Database Foundation               | 🔴 Blocked               | [PHASE_1_SCOPE.md](PHASE_1_SCOPE.md) — scope determined, blocked on WSL2, see `EXECUTION_STATE.md` |
| 2     | Organization & Project Foundation | 🟡 Approved, not started | [PHASE_2_SCOPE.md](PHASE_2_SCOPE.md) — scope approved in principle, implementation not started     |
| 3     | Authentication & API Keys         | ⬜ Not started           | —                                                                                                  |
| 4     | Gateway & Routing                 | ⬜ Not started           | —                                                                                                  |
| 5     | Provider Adapter System           | ⬜ Not started           | —                                                                                                  |
| 6     | Usage, Cost & Metrics             | ⬜ Not started           | —                                                                                                  |
| 7     | Organization & RBAC (Advanced)    | ⬜ Not started           | —                                                                                                  |
| 8     | SDKs                              | ⬜ Not started           | —                                                                                                  |
| 9     | Reliability / Observability       | ⬜ Not started           | —                                                                                                  |
| 10    | Production Hardening              | ⬜ Not started           | —                                                                                                  |
| 11    | Integration / E2E                 | ⬜ Not started           | —                                                                                                  |
| 12    | Final Production Readiness        | ⬜ Not started           | —                                                                                                  |

Legend: ⬜ not started · 🟡 in progress / proposed · 🟢 complete (acceptance gate passed) · 🔴 blocked

## Known environment constraints affecting future phases

- **Docker Desktop installed but non-functional — needs WSL2, needs a human at the machine.** `winget install Docker.DockerDesktop` succeeded, but Docker Desktop has no backend to run on: this machine has never had WSL enabled, and enabling it (`wsl --install`) requires an elevated/interactive session no automated shell can provide. Blocks Phase 1 until the user runs `wsl --install` from an Administrator shell and restarts Windows.
- **No Python on the dev machine** — only blocks re-running `specs/schemas/_verify_all.py`; does not block any phase's implementation work.
- **No C compiler** — blocks Go's `-race` detector specifically. Does not block normal `go build`/`go test`/`go vet`.
- **Phase 3 (Authentication) will need OIDC/SSO provider decisions and credentials**; **Phase 5 (Provider Adapters) will need real LLM provider API keys** for anything beyond contract-shape tests. These will be raised as explicit blockers when those phases start, not assumed or fabricated.
- **Phase 7 (Organization & RBAC, Advanced) may be thin as currently specified** — `specs/` only defines a flat 4-role enum (`User.role`), no fine-grained permission model to build "advanced" RBAC from. See `PHASE_2_SCOPE.md` section H.
- **Two entity clusters in `specs/database/entities.md` have no owning phase anywhere in this 13-phase roadmap**, found while determining Phase 1's scope (`PHASE_1_SCOPE.md` section H): the Agent domain (`AgentSession`, `AgentMemory` — orphaned since Revision 2 dropped every agent-runtime phase, but the database spec was never updated to match) and eventing/NATS integration (no phase mentions it, despite `specs/events/*.md` and `docker-compose.yml` both already assuming it exists; `EventLog` is the most directly affected table). Neither blocks any currently-scoped phase; both are `specs/`-level or roadmap-level gaps to resolve separately, not acted on yet.

## Remote repository state (for context on future sessions)

`github.com/3boalazm/ai-dos`:

- `main` (default branch) — this repository's history, currently at commit `d0aa4f4` (matches local at time of writing).
- `foundation` — same content as `main` as of the replacement; an artifact of the push sequence, left in place, not required for anything going forward.
- `legacy-before-ai-dos` — the prior unrelated Python codebase that used to occupy `main`, preserved exactly, untouched, not merged. See "Roadmap revision history" above.
