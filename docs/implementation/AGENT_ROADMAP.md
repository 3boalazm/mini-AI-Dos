# Agent Expansion Roadmap

Status: **approved working track** (2026-08-09, user decision). This is a parallel expansion track alongside the platform roadmap in [ROADMAP_STATUS.md](ROADMAP_STATUS.md) — it does not renumber or replace those phases. The guiding rule, in the user's words: single-agent loop first, phase by phase, each phase adding one real capability. **No multi-agent until the single loop is reliably doing Plan → Build → Inspect → Fix.**

## Phases

| #   | Name                    | Capability added                                                                                                                                                                                                               | Status                                          |
| --- | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------- |
| A1  | Core Agent Loop         | Understand → Plan → Execute → Inspect → Fix → Result, with observable statuses and user-visible steps. No tools — execution is model-only.                                                                                     | 🟢 Done                                         |
| A2  | File System + Workspace | Per-run isolated workspace; tools read_file / write_file / edit_file / list_files / search_files via a text JSON protocol; execute + fix run the tool loop; file tree + sandboxed preview in /chat. No delete / install / git. | 🟢 Done                                         |
| A3  | Terminal                | run command / process management inside the run workspace                                                                                                                                                                      | ⬜                                              |
| A4  | Browser                 | open / inspect / screenshot — the agent sees its own work (Build → See → Critique → Fix)                                                                                                                                       | ⬜                                              |
| A5  | Task Events UI          | task events only (Planning / Writing files / Running / Testing / Found issue / Fixing / Completed) — no internal reasoning dumps                                                                                               | 🟡 partial: live per-step events exist in /chat |
| A6  | Project Workspace       | persistent project context; chat bound to a project                                                                                                                                                                            | ⬜                                              |
| A7  | Simple Planner          | plan shown before execution with [Start] gate                                                                                                                                                                                  | ⬜                                              |
| A8  | Approval System         | safe actions auto-run; sensitive ones (delete, install, commit, deploy, env changes) ask Allow once / Always / Deny                                                                                                            | ⬜                                              |
| A9  | Error Recovery          | build failed → read error → locate → fix → re-run → report summary only                                                                                                                                                        | ⬜                                              |
| A10 | Git / Versioning        | changes summary, commit / revert / compare                                                                                                                                                                                     | ⬜                                              |

V2 (explicitly deferred): memory, parallel tasks, better browser agent, deployment, multi-agent, advanced integrations.

## Architecture decisions (A1, load-bearing)

- **The loop lives in the gateway** (`services/gateway/internal/agent`), reusing the existing `provider.Provider` seam — the agent inherits every provider the stack gains in platform Phase 4/5 for free.
- **Statuses are the contract**: `planning → executing → inspecting → fixing → completed | failed`. The /chat five-state UI maps onto them without rework (working card = live run card). Later phases add step _kinds_, never a new lifecycle.
- **HTTP surface**: `POST /v1/agent/runs` (create, auth + rate limited), `GET /v1/agent/runs/{id}` (poll snapshot, auth), `POST /v1/agent/runs/{id}/cancel` (real cancellation via context). Polling, not SSE — smallest thing that works; SSE is an A5 refinement.
- **Runs are in-memory and ephemeral** — the platform's database phase is still blocked (WSL2); when it lands, runs gain persistence without API changes.
- **A2 tools execute in a per-run workspace inside the container** (`AGENT_WORKSPACE_DIR`, default temp), not on the caller's machine. Path safety is the one security boundary: `Workspace.resolve` rejects absolute paths (cross-platform: `filepath.IsAbs`, leading separators, and volume names), rejects any `..` component, and re-verifies containment with `filepath.Rel`. Only 5 read/write tools exist — no delete/install/git until the approval system (A8) lands. Generated files are served at `GET .../files/{path}` as `text/plain` with `nosniff` and previewed only in a sandboxed iframe, so agent-authored markup never executes in the gateway's origin.
- **Tools use a text JSON protocol, not native tool-calling** — the provider seam is plain chat, and a text protocol is portable across every backend in the free stack (which each have different native formats). Model calls per run are bounded: ≤5 plan steps × ≤6 tool calls, plus one fix loop.

## Relationship to the platform roadmap

Revision 2 of the platform roadmap dropped agent scope because `specs/` did not define it. This track reinstates it as a **user-approved product decision** logged here; `specs/` for the agent surface (schemas, contracts) are a debt this track owes and should produce as phases mature.
