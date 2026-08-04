# AI-DOS Roadmap Status

Authoritative, must reflect reality — never mark a phase done here without a corresponding `PHASE_<N>_REPORT.md` whose Definition of Done checklist is fully satisfied. See `EXECUTION_STATE.md` for the live in-progress detail and `ARCHITECTURE.md` for the as-built system shape.

| Phase | Name                       | Status         | Report                                                             |
| ----- | -------------------------- | -------------- | ------------------------------------------------------------------ |
| 0     | Baseline Stabilization     | 🟢 Complete    | [PHASE_0_REPORT.md](PHASE_0_REPORT.md), commit `9251a7a`           |
| 1     | Database Foundation        | 🔴 Blocked     | Docker Desktop installed but needs WSL2 — see `EXECUTION_STATE.md` |
| 2     | Provider Platform          | ⬜ Not started | —                                                                  |
| 3     | Identity & Access          | ⬜ Not started | —                                                                  |
| 4     | Gateway & Routing          | ⬜ Not started | —                                                                  |
| 5     | Agent Runtime              | ⬜ Not started | —                                                                  |
| 6     | Secure Tool Runtime        | ⬜ Not started | —                                                                  |
| 7     | Memory & Context           | ⬜ Not started | —                                                                  |
| 8     | Orchestration & Subagents  | ⬜ Not started | —                                                                  |
| 9     | Channels & Voice           | ⬜ Not started | —                                                                  |
| 10    | Production & Observability | ⬜ Not started | —                                                                  |

Legend: ⬜ not started · 🟡 in progress · 🟢 complete (acceptance gate passed) · 🔴 blocked

## Known environment constraints affecting future phases

- **Docker Desktop installed but non-functional — needs WSL2, needs a human at the machine.** `winget install Docker.DockerDesktop` succeeded, but Docker Desktop has no backend to run on: this machine has never had WSL enabled, and enabling it (`wsl --install`) requires an elevated/interactive session no automated shell can provide. Blocks Phase 1 until the user runs `wsl --install` from an Administrator shell and restarts Windows.
- **No Python on the dev machine** — only blocks re-running `specs/schemas/_verify_all.py`; does not block any phase's implementation work.
- **No C compiler** — blocks Go's `-race` detector specifically. Does not block normal `go build`/`go test`/`go vet`.
- **Phase 2/3/7/9 will need real external credentials** the user must provision (LLM provider API keys, OAuth app registrations for SSO providers, a vector DB instance, messaging-platform credentials). These will be raised as explicit blockers when those phases start, not assumed or fabricated.
