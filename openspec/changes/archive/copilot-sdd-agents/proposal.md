# Proposal: Ship Copilot SDD Custom Agents

## Intent

capiko's SDD phase skills already predicate STRICT delegation — every skill carries a `## Gate` ("Orchestrator: DELEGATE to a fresh sub-agent... running phase work yourself is an orchestration error"). But capiko ships NO Copilot custom agents, so on the Copilot side there is **no delegation target**: the gate dangles. Live spikes on Copilot CLI v1.0.59 (obs #57, #58) verified that global `~/.copilot/agents/*.agent.md` reading, explicit `--agent` selection, body injection, description inference, and coordinator→worker delegation via the `agent` tool + `agents:` allowlist all work. This change ships a catalog of Copilot custom agents for the SDD phases and installs them to `~/.copilot/agents/`, **materializing** the strict delegation the skills already assume.

## Scope

### In Scope
- `AgentsDir` (`~/.copilot/agents`) on `copilot.Host` + `InstalledAgents`/`UninstallAgent`, mirroring the existing `SkillsDir`/`InstalledSkills`/`UninstallSkill` machinery.
- Embedded `.agent.md` catalog: one thin WORKER per SDD phase (explore/propose/spec/design/tasks/apply/verify/archive), `user-invocable: false`, body pointing at `~/.copilot/skills/<phase>/SKILL.md` + `~/.copilot/skills/sdd-shared/`; plus one COORDINATOR agent.
- Install / sync / drift paths through `internal/skill` + `internal/catalog` + the TUI, parallel to the skills flow.
- Q2 strict routing: coordinator routes by the NATIVE ENGINE (`capiko-ai sdd-status --json` → `nextRecommended`) and delegates EXPLICITLY to that phase's worker via the `agent` tool + `agents:` allowlist (not description inference for critical routing).
- Q3 unified language contract carried in the agents (reply-to-human in the human's language; all artifacts/handoffs in English).
- `tools` mapped to Copilot aliases (read/edit/search/execute/agent). NO Anthropic model aliases in `model:` — omit the field (inherit default) or use a valid Copilot model name.

### Out of Scope
- Q4 interactive proposal round (a "Step 0") — separate future change.
- Per-phase model-assignment debt (#22/#23) beyond noting workers must not use Anthropic aliases.
- judgment-day (jd-*) agents — SDD phases only.

## Capabilities

### New Capabilities
- `copilot-sdd-agents`: install/sync/uninstall an embedded catalog of SDD-phase Copilot custom agents (`.agent.md`) into `~/.copilot/agents/`, with drift detection and a coordinator that routes via the native engine.

### Modified Capabilities
- None (no existing spec files; `openspec/specs/` is empty).

## Approach

Mirror the proven skills pipeline. Add `AgentsDir` + agent enumeration to `copilot.Host`. Embed an agent catalog (Go `embed`) the same way skills are embedded. Workers are THIN: frontmatter (`description`, `tools`, `user-invocable: false`) + a body that defers to the already-installed skill files, so the skill remains the single source of phase logic. The coordinator (decided: ship a dedicated coordinator over relying on `/fleet`, since the spike confirmed a custom coordinator + `agents:` allowlist works and gives us deterministic, native-engine-driven routing) reads `sdd-status --json` and explicitly invokes the next worker. The language contract lives in the coordinator and is referenced by workers.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/copilot/copilot.go` | Modified | Add `AgentsDir`, `InstalledAgents`, `UninstallAgent` (mirror skills) |
| `internal/catalog/` | Modified | Embed + expose the agent catalog |
| `internal/skill/` | Modified | Install/sync/drift logic for agents alongside skills |
| `internal/tui/` | Modified | Install/sync/drift screens surface agents |
| `assets/agents/*.agent.md` | New | Embedded worker + coordinator agents |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Agent body drifts from referenced skill paths | Med | Body references skill paths (no logic copy); drift check validates referenced skills are installed |
| Invalid Copilot `model:` value warns/falls back | Med | Omit `model:` to inherit default; document valid names as config, not mechanism |
| Copilot custom-agent CLI maturity (`tools` caveats) | Low | All paths empirically verified on v1.0.59 (obs #57/#58); keep agents thin |
| Coordinator routing diverges from skills' gate | Low | Route by native engine `nextRecommended`, single source of truth |

## Rollback Plan

Agents are additive and isolated under `~/.copilot/agents/`. Revert the PR(s); `UninstallAgent` (idempotent, refuses non-`.agent.md`) removes installed files. No change to skills, instructions, or state schema, so existing SDD flows are unaffected.

## Dependencies

- Copilot CLI present (existing `Detect()` already gates on `~/.copilot`).
- Installed SDD-phase skills (workers reference `~/.copilot/skills/<phase>/SKILL.md`).
- `capiko-ai sdd-status --json` native engine (already shipped, PRs #45–#48).

## Success Criteria

- [ ] `~/.copilot/agents/` populated with one worker per SDD phase + a coordinator after install.
- [ ] `copilot --agent <coordinator>` delegates to the native-engine-selected phase worker.
- [ ] `InstalledAgents`/`UninstallAgent` behave idempotently, mirroring skills.
- [ ] Drift detection reports missing/changed agents.
- [ ] `go test -race ./...`, `gofmt -l .` (empty), `go vet ./...` all green.
