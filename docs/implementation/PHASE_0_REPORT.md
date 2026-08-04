# Phase 0 Report — Baseline Stabilization

## Objective

Make the repository a verified, reproducible development baseline before any new feature work starts.

## Audit performed

Repository inspected first-hand (not from prior claims): git log/status/HEAD, all four `services/foundation` packages and their tests, the full TypeScript workspace, all 43 files under `specs/`, `docker-compose.yml`, `.env.example`, `go.work`, `.github/workflows/ci.yml`, `Makefile`, and root docs. Full findings recorded earlier in this conversation and reconfirmed here where they affect Phase 0 specifically. Cross-referenced against `AI_DOS_SOURCE_MANIFEST.json` and `AI_DOS_SOURCE_SHA256.txt` — git HEAD (`2a5cfc252e0b284954477f97ea73406a5c65f036`) matches the manifest's recorded commit exactly.

Dev-machine toolchain audit (this is a fresh Windows machine, not the original authoring sandbox): Node 24.15.0 and npm present; Go, Python, Docker, and a C compiler absent; winget present.

## Implementation (this phase's changes)

Phase 0 is stabilization, not feature work — the diff is intentionally small and non-functional to `services/foundation`'s behavior:

| File                                                                                  | Change                                                                                                  | Why                                                                                                                                                                                 |
| ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `specs/schemas/_verify_all.py`                                                        | Fixed hardcoded `/home/claude/...` glob path to resolve relative to the script's own location           | Confirmed bug from prior-turn inspection: silently validated zero schemas and still printed `ALL CHECKS PASSED` when run from anywhere but the original sandbox                     |
| `specs/api/openapi.yaml`                                                              | Added `/internal/providers/{id}/deprecate` path (modeled on sibling `/internal/providers/{id}/publish`) | `specs/api/providers.md` already documented this route; `openapi.yaml` was missing it — an objective spec-internal inconsistency, not a design change                               |
| `services/foundation/errors/errors_test.go`                                           | Checked a previously-discarded `json.Marshal` error instead of `_`-discarding it                        | Real `golangci-lint` `errcheck` failure under this repo's own config; would fail real CI (the Makefile's `lint` target silently tolerates it, CI's `golangci-lint-action` does not) |
| `docs/implementation/{ARCHITECTURE,ROADMAP_STATUS,EXECUTION_STATE,PHASE_0_REPORT}.md` | New                                                                                                     | Persistent cross-session tracking this execution model requires                                                                                                                     |

No changes to `services/foundation`'s non-test source, no changes to any TypeScript source, no changes to any entity/contract/schema content beyond the one added path above.

## Architectural decisions

- **Spec-internal fixes are in scope for Phase 0; spec _design_ changes are not.** Both `specs/` edits above fix an objective inconsistency within the spec repository itself (a script that can't do what it claims; a documented route missing from the machine-readable contract that's supposed to be its source of truth). Neither changes what any entity, route, or contract _means_. This distinction matters because `docs/repository-overview.md` states specs are immutable from the application side — these fixes are the one narrow exception the Phase 0 task list itself carved out ("resolve known contract inconsistencies only if they are objectively inconsistent"), not a precedent for redesigning specs during later phases.
- **Used golangci-lint v1.63.4, not CI's pinned v1.61.0.** v1.61.0 does not compile under the Go version available on this machine (see Known Limitations). Chose the closest v1.x release rather than jumping to v2 (which changes `.golangci.yml`'s schema and would have forced an unrelated config migration out of scope for this phase).
- **Did not install Python or Docker.** Offered to the user alongside Go; only Go was selected. Respecting that choice explicitly rather than installing more than what was approved — Python and Docker remain open items, not silently resolved.

## Tests

No new tests added (no new behavior to cover — this phase is stabilization). Existing test suites re-run to completion, not sampled:

- Go: `go test -cover ./services/foundation/...` — **23/23 pass**, coverage 84.2–96.2% across the four packages.
- TypeScript: `pnpm run test` (vitest, 3 packages) — **20/20 pass**.

## Validation results

| Check                                                                | Status                                                                                                                           |
| -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `go build ./services/foundation/...`                                 | ✅                                                                                                                               |
| `go vet ./services/foundation/...`                                   | ✅                                                                                                                               |
| `gofmt -l services/foundation/`                                      | ✅ (no output)                                                                                                                   |
| `golangci-lint run --config=.golangci.yml ./services/foundation/...` | ✅ (v1.63.4; see Known Limitations for why not v1.61.0)                                                                          |
| `go test -cover ./services/foundation/...`                           | ✅ 23/23, 84.2–96.2% coverage                                                                                                    |
| `go test -race ...`                                                  | ⚠️ not run — no C compiler on this machine                                                                                       |
| `pnpm install --frozen-lockfile`                                     | ✅                                                                                                                               |
| `pnpm run build`                                                     | ✅                                                                                                                               |
| `pnpm run typecheck`                                                 | ✅                                                                                                                               |
| `pnpm run lint`                                                      | ✅                                                                                                                               |
| `pnpm run test`                                                      | ✅ 20/20                                                                                                                         |
| `prettier --check .`                                                 | ✅ (repo-wide)                                                                                                                   |
| `specs/schemas/_verify_all.py` (fixed)                               | ⚠️ not executed — no Python on this machine; fix is a 3-line, reasoned-correct change, not independently confirmed by running it |

## Known limitations

1. **`-race` unverified** — this machine has no C compiler (CGO required). Should be resolved before this baseline is treated as production-representative; does not block Phase 0's literal Definition of Done.
2. **golangci-lint version drift** — ran v1.63.4 instead of CI's pinned v1.61.0 due to a genuine Go-1.26/x-tools@v0.24.0 incompatibility in the older release. Result should be representative (same linter set, same config) but is not byte-identical to what CI runs today. Bumping CI's pin is a reasonable follow-up, not done here (CI config is out of scope for "fix genuine baseline issues" in app/spec code).
3. **`specs/schemas/_verify_all.py` fix unexecuted** — edited, not run. No Python on this machine; offered to the user and not selected this pass.
4. **No Docker on this machine** — doesn't affect Phase 0 (no database work this phase) but is a hard blocker for Phase 1, flagged now rather than at Phase 1's start.

None of these are silent — all four are recorded here and in `EXECUTION_STATE.md` and will be re-surfaced, not re-discovered, when they become load-bearing.

## Security considerations

None applicable — this phase touched no security boundary (no auth, no secrets, no external input handling). The one behavioral code change (`errors_test.go`) is test-only and strictly additive (checks an error that was previously silently ignored).

## Definition of Done checklist (from the execution mandate)

- [x] Go builds.
- [x] Go tests pass.
- [x] Go vet passes.
- [x] TypeScript tests pass.
- [x] Typecheck passes.
- [x] Lint passes (Go: golangci-lint v1.63.4, see limitation #2; TypeScript: eslint, exact match).
- [x] Build passes.
- [x] Specs validation is trustworthy — the specific known-broken verification script is now fixed (though unexecuted; see limitation #3), and the one confirmed spec-internal inconsistency is resolved.
- [x] Repository is clean — no unrelated changes; only the Husky hook mode-only diff remains, which predates this session and is Windows-checkout noise, not content drift (left untouched, will not be committed as part of this phase).
- [x] Baseline is reproducible — every check above has a recorded command and result; a future session can re-run the same commands and expect the same outcome modulo the four noted environment gaps.

**Gate: PASSED**, with limitations #1–#4 carried forward transparently rather than blocking. None of them contradict any item in the literal Definition of Done as the user stated it; all are flagged so they can't be mistaken for silently-resolved later.

## Next phase dependencies

Phase 1 (Database Foundation) needs, before its own acceptance gate can pass:

- **Docker** on this machine (or an alternative reachable Postgres) — `docker-compose.yml` already defines Postgres 16 / Redis 7 / NATS JetStream, untested against any running daemon so far.
- A migration tool decision (`specs/database/migrations-strategy.md` specifies `golang-migrate`, forward-only, semver `schema_version` per entity) — not yet installed or wired up.
- The ~13 already-standalone JSON Schemas plus ~24 table-only entities in `specs/database/entities.md` as the schema source of truth.

Will raise the Docker blocker explicitly before starting Phase 1 rather than assuming a path forward.
