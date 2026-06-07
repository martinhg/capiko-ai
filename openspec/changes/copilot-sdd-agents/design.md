# Design: Ship Copilot SDD Custom Agents

## Technical Approach

Mirror the proven skills pipeline end-to-end. The `agent` is a new domain type that
parallels `skill.Skill` but is a single `.agent.md` file (no `Extra`, no directory).
A second embedded catalog (`internal/catalog/agents/`, sibling of `skills/`) feeds
`copilot.Host` methods (`AgentsDir`/`InstalledAgents`/`UninstallAgent`) that mirror
the existing skills equivalents exactly. Install/sync/drift reuse the same
`state.Checksum` + `state.Store` machinery via a parallel `state.Agents` map. The
TUI sync writes agents alongside skills. Workers stay THIN (defer to installed
`SKILL.md`); the coordinator routes via the native engine. Honors proposal
decisions: dedicated coordinator, `model:` omitted, Copilot tool aliases, explicit
native-engine routing (Q2), unified language contract (Q3).

## Architecture Decisions

| Decision | Choice | Rejected alternative | Rationale |
|---|---|---|---|
| Embed location | `internal/catalog/agents/` (flat `*.agent.md`), sibling of `skills/` | Top-level `assets/agents/` | Keeps embed wiring identical to `catalog.go`; no new package layout; proposal's `assets/agents/` table row was indicative, not binding. |
| Agent type | New `agent.Agent` package (single-file model) reusing `frontmatter` parsing pattern | Reuse `skill.Skill` with empty `Extra` | A skill is a *directory* with `SKILL.md`; an agent is a *single file*. Forcing the directory model would distort `Install`/`InstalledSkills` semantics and the `.agent.md` filename convention. A thin parallel type is clearer than an overloaded one. |
| Frontmatter parsing | Extract shared YAML-block splitter; agent parses richer fields (`name`,`description`,`tools`,`user-invocable`,`agents`,`target`) | Duplicate `skill.frontmatter` | Agents need fields skills don't. Share the delimiter logic, diverge on the `meta` struct. |
| Checksum/drift | Reuse `state.Checksum(canonical)`; add `state.Agents map[string]AgentRecord` + `drift.StaleAgents` | New checksum scheme | Drift detection is already content-hash based; agents are content too. One mechanism, two maps. |
| Routing source | Coordinator runs `capiko-ai sdd-status --json`, reads `nextRecommended`; **post-tasks** phases (apply/verify/archive) route by native engine. Planning phases (explore→tasks) the engine does not status-route — coordinator advances the documented DAG order, confirming each artifact exists. | Description inference for all routing | Native engine only emits `apply`/`verify`/`archive`/`resolve-blockers` (verified in `status.go:resolveNextRecommended`). Inference is non-deterministic (obs #59). Explicit routing where the engine speaks; deterministic DAG fallback where it is silent. |
| Coordinator vs `/fleet` | Dedicated coordinator agent | Rely on `/fleet` | Spike (obs #58) confirmed custom coordinator + `agents:` allowlist gives deterministic, native-engine-driven routing. |

## Data Flow

```
   embed FS (internal/catalog/agents/*.agent.md)
        │  catalog.LoadAgents()
        ▼
   []agent.Agent ──► RunSync ──► Agent.Install(host.AgentsDir) ──► ~/.copilot/agents/<name>.agent.md
        │                            │
        │                            └─► state.Store.ApplyAgents (checksum)
        ▼
   drift.StaleAgents(catalog, state)  ──► TUI sync/drift screens

   Runtime (Copilot CLI):
   copilot --agent capiko-sdd-coordinator
        │ runs: capiko-ai sdd-status --json → nextRecommended
        ▼ agent tool (agents: allowlist)
   capiko-sdd-<phase> worker ──► reads ~/.copilot/skills/sdd-<phase>/SKILL.md + sdd-shared/
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/agent/agent.go` | Create | `Agent{Name,Description,Content}`, `Install(agentsDir)` (writes `<name>.agent.md`), `CanonicalContent()`, `LoadCatalog(fs.FS)`, frontmatter parse (`tools`,`user-invocable`,`agents`,`target`). |
| `internal/agent/agent_test.go` | Create | Unit + table tests. |
| `internal/catalog/agents/*.agent.md` | Create | 8 workers (`capiko-sdd-{explore,propose,spec,design,tasks,apply,verify,archive}`) + 1 coordinator (`capiko-sdd-coordinator`). |
| `internal/catalog/catalog.go` | Modify | Add `//go:embed agents` + `LoadAgents() ([]agent.Agent, error)`. |
| `internal/copilot/copilot.go` | Modify | Add `AgentsDir` field (set in `Detect`), `InstalledAgents()`, `UninstallAgent(name)`. |
| `internal/state/state.go` | Modify | Add `Agents map[string]AgentRecord`, `AgentRecord`, `ApplyAgents`; init nil-map in `Load`. |
| `internal/drift/drift.go` | Modify | Add `StaleAgents(catalog []agent.Agent, st *state.State) []string`. |
| `internal/tui/sync.go` | Modify | `RunSync` also installs agents + records `ApplyAgents`; view counts agents. |
| `internal/tui/app.go` | Modify | `NewApp`/`services` carry agent catalog; drift surfaces stale agents. |

## Interfaces / Contracts

```go
// internal/copilot/copilot.go — mirror the skills trio exactly.
func (h *Host) InstalledAgents() (map[string]bool, error)   // *.agent.md stems present
func (h *Host) UninstallAgent(name string) error            // idempotent; refuses non-*.agent.md

// internal/agent/agent.go
type Agent struct{ Name, Description, Content string }       // Name = filename stem (kebab)
func (a Agent) Install(agentsDir string) (string, error)    // writes <name>.agent.md, returns path
func (a Agent) CanonicalContent() string                    // == Content (single file)
func LoadCatalog(fsys fs.FS) ([]Agent, error)               // reads *.agent.md, sorted by name
```

`AgentsDir = filepath.Join(cfg, "agents")`. `InstalledAgents` enumerates files
matching `*.agent.md` (not directories — the only structural difference from
`InstalledSkills`). `UninstallAgent` guard: refuse names not ending `.agent.md`
target / non-regular file, mirroring the SKILL.md guard.

## Agent Content Design

**Worker** (`capiko-sdd-design.agent.md`) — one per phase:

```markdown
---
description: "SDD design phase executor. Decides technical approach and architecture for the active change."
tools: ['read', 'edit', 'search', 'execute']
user-invocable: false
---
You are the capiko SDD **design** executor. Do this phase only; do NOT delegate.
Read and follow EXACTLY: ~/.copilot/skills/sdd-design/SKILL.md
Shared contract: ~/.copilot/skills/sdd-shared/sdd-phase-common.md
Language: reply to the human in the human's language; ALL artifacts and handoffs in English.
```

(`tools` per phase: planning workers `read,edit,search`; apply/verify add `execute`.
No `agent` tool on workers — they never delegate. No `model:` field.)

**Coordinator** (`capiko-sdd-coordinator.agent.md`):

```markdown
---
description: "SDD coordinator. Routes each phase to its dedicated worker by the native engine."
tools: ['execute', 'read', 'agent']
agents: ['capiko-sdd-explore','capiko-sdd-propose','capiko-sdd-spec','capiko-sdd-design','capiko-sdd-tasks','capiko-sdd-apply','capiko-sdd-verify','capiko-sdd-archive']
user-invocable: true
---
You are the capiko SDD coordinator. You COORDINATE; you do not execute phases.
Language Domain Contract: reply to the human in the human's language; ALL artifacts and inter-agent handoffs in English. (Single source — workers reference this.)

Routing algorithm (deterministic):
1. Run `capiko-ai sdd-status --json` in the repo. Parse `nextRecommended`.
2. If `nextRecommended` is `apply` | `verify` | `archive`: delegate to `capiko-sdd-<that>` via the agent tool.
3. If `resolve-blockers`: read `blockedReasons`; advance the planning DAG in order
   (proposal → spec/design → tasks), delegating to the first phase whose artifact is missing.
4. For a brand-new change (no proposal): delegate `capiko-sdd-explore` then `capiko-sdd-propose`.
5. After each worker returns, re-run step 1 until `nextRecommended` is `archive` done.
Never run phase work yourself. Delegate ONLY to agents in the allowlist.
```

## Language Contract Placement (Q3)

The unified contract lives **once** in the coordinator body ("Language Domain
Contract"). Each worker carries a one-line `Language:` echo that points to the same
rule, so a worker invoked directly (`--agent capiko-sdd-design`) still has it, but
the coordinator is the canonical source. No duplication of the full contract.

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | `agent.LoadCatalog` parses all 9 embedded agents; names, descriptions, `user-invocable`, `agents` allowlist correct | Table test over embedded FS |
| Unit | `Agent.Install` writes `<name>.agent.md`, returns path, content verbatim; `CanonicalContent==Content` | tmp dir |
| Unit | `Host.InstalledAgents` enumerates only `*.agent.md`; `UninstallAgent` idempotent + refuses bad names | tmp `AgentsDir`, test seams |
| Unit | `drift.StaleAgents` flags changed checksum, ignores untracked/missing | reuse drift_test pattern |
| Golden/validation | Each catalog agent: valid frontmatter, `description` non-empty, workers `user-invocable:false` + no `agent` tool, coordinator allowlist == worker set, **no `model:` field**, **no Anthropic alias** anywhere | catalog-level test asserting invariants |
| Validation | Worker bodies reference an existing `~/.copilot/skills/sdd-<phase>/` path that maps to a real catalog skill | cross-check against `catalog.Load()` names |
| Unit | `RunSync` installs agents + records `ApplyAgents`; count includes agents | extend sync_test |

Test command: `go test -race ./...`; `gofmt -l .` empty; `go vet ./...` green.

## Migration / Rollout

No migration. Additive: new `Agents` map in state defaults to empty; existing
state.json loads unchanged. Agents are isolated under `~/.copilot/agents/`.

## Review Workload (feeds sdd-tasks)

Estimate ~350–450 changed lines incl. tests + 9 agent files. Near the 400-line
budget. Recommended slices for `sdd-tasks`:
1. **Domain + catalog**: `internal/agent/` + `catalog.LoadAgents` + the 9 `.agent.md` files + their tests.
2. **Host + state + drift**: `copilot.Host` agent methods, `state.Agents`/`ApplyAgents`, `drift.StaleAgents` + tests.
3. **TUI wiring**: `RunSync`/`app.go` agent install + drift surfacing + tests.

If slice 1's agent-file content pushes it over budget, split the `.agent.md`
authoring into its own PR (pure content, trivially reviewable).

## Open Questions

- [ ] Exact valid Copilot `model:` names per account — out of scope (config, not mechanism; `model:` omitted by design).
- [ ] Whether to add agents as a *separate* TUI menu entry vs. folding into the existing sync/drift screens — design folds in (least surface); revisit if UX wants explicit visibility.
