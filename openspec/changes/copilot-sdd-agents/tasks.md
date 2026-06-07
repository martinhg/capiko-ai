# Tasks: Ship Copilot SDD Custom Agents

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 420–500 (Go code + tests + 9 `.agent.md` files) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Slice 1) → PR 2 (Slice 2) → PR 3 (Slice 3) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | `internal/agent` package + 9 embedded `.agent.md` files + `catalog.LoadAgents` | PR 1 | Base: `main`. Self-contained domain layer; all content validated by tests. Rollback: revert PR 1. |
| 2 | `copilot.Host` agent methods + `state.Agents` map + `drift.StaleAgents` | PR 2 | Base: PR 1 branch. Host and persistence layer. Rollback: revert PR 2, PR 1 stays. |
| 3 | TUI wiring (`RunSync`, `app.go`) + `staleBanner` agents section | PR 3 | Base: PR 2 branch. Surface layer only; no domain changes. Rollback: revert PR 3. |

---

## Slice 1 — Domain Layer: `internal/agent` + Catalog Content + `catalog.LoadAgents`

> **Spec requirements covered**: EmbeddedAgentCatalog, WorkerAgentShape, CoordinatorRouting, LanguageContractInAgents  
> **Verification**: `go test -race ./internal/agent/... ./internal/catalog/...`; `gofmt -l .`; `go vet ./...`

### Phase 1.1 — RED: Agent domain type tests

- [x] 1.1 Create `internal/agent/agent_test.go` with failing tests:
  - `TestLoadCatalog_ReturnsNineAgents` — asserts `LoadCatalog` returns exactly 9 entries (spec: EmbeddedAgentCatalog / Scenario: Catalog is loaded).
  - `TestLoadCatalog_MalformedAgent_ReturnsError` — asserts malformed frontmatter yields non-nil error (spec: EmbeddedAgentCatalog / Scenario: Malformed embedded agent file).
  - `TestAgent_Install_WritesFile` — asserts `Install(agentsDir)` creates `<name>.agent.md`, returns the path, content verbatim (spec: AgentInstallAndSync / Scenario: First install).
  - `TestAgent_Install_NoopOnIdenticalContent` — asserts mtime unchanged when content matches (spec: AgentInstallAndSync / Scenario: Re-install with identical content).
  - `TestAgent_Install_OverwritesOnDrift` — asserts overwrite when content differs (spec: AgentInstallAndSync / Scenario: Re-install after content drift).
  - `TestCanonicalContent_EqualsContent` — asserts `CanonicalContent() == Content`.

### Phase 1.2 — RED: Catalog validation tests

- [x] 1.2 Extend `internal/agent/agent_test.go` with golden/catalog-level failing tests:
  - `TestCatalog_WorkerFrontmatter` — table over all 8 workers: `user-invocable: false`; `tools` only in `{read,edit,search,execute,agent}`; no `model:` Anthropic alias (spec: WorkerAgentShape / Scenario: Worker frontmatter validation).
  - `TestCatalog_WorkerBodyReferencesSkillPath` — each worker body contains `~/.copilot/skills/sdd-<phase>/SKILL.md` (spec: WorkerAgentShape / Scenario: Worker body references skill path).
  - `TestCatalog_CoordinatorAllowlist` — coordinator `agents:` list equals the 8 worker names exactly (spec: CoordinatorRouting / Scenario: Coordinator frontmatter allowlist).
  - `TestCatalog_CoordinatorBodyCitesNativeEngine` — coordinator body contains `capiko-ai sdd-status --json` and `nextRecommended` (spec: CoordinatorRouting / Scenario: Coordinator body cites native engine).
  - `TestCatalog_CoordinatorBodyExplicitDelegation` — coordinator body references the `agent` tool for delegation (spec: CoordinatorRouting / Scenario: Coordinator body mandates explicit delegation).
  - `TestCatalog_LanguageContract_Coordinator` — coordinator body contains language contract markers (spec: LanguageContractInAgents / Scenario: Coordinator carries language contract).
  - `TestCatalog_LanguageContract_Workers` — each worker body contains a language contract line (spec: LanguageContractInAgents / Scenario: Worker carries language contract).

### Phase 1.3 — GREEN: Implement `internal/agent/agent.go`

- [x] 1.3 Create `internal/agent/agent.go`:
  - `type Agent struct { Name, Description, Content string }` — kebab filename stem, parsed description, raw file content.
  - `func (a Agent) CanonicalContent() string` — returns `a.Content` verbatim (single-file model; no directory structure).
  - `func (a Agent) Install(agentsDir string) (string, error)` — `os.MkdirAll(agentsDir, 0o755)`; check existing file checksum (`state.Checksum`); skip write if equal; else `os.WriteFile("<name>.agent.md")`; return written path.
  - `func LoadCatalog(fsys fs.FS) ([]Agent, error)` — `fs.Glob(fsys, "*.agent.md")`; parse frontmatter YAML block; populate `Name` (stem), `Description`, `Content`; sort by name; return error if any frontmatter is invalid.
  - Frontmatter parser: extract YAML between `---` delimiters; unmarshal into local struct with `description`, `tools`, `user-invocable`, `agents`, `target` fields.

### Phase 1.4 — GREEN: Author the 9 embedded `.agent.md` files

- [x] 1.4 Create `internal/catalog/agents/capiko-sdd-explore.agent.md` — worker; `tools: [read,edit,search]`; `user-invocable: false`; no `model:`; body references `~/.copilot/skills/sdd-explore/SKILL.md` + one-line language echo.
- [x] 1.5 Create `internal/catalog/agents/capiko-sdd-propose.agent.md` — same pattern as 1.4; references `sdd-propose/SKILL.md`.
- [x] 1.6 Create `internal/catalog/agents/capiko-sdd-spec.agent.md` — references `sdd-spec/SKILL.md`.
- [x] 1.7 Create `internal/catalog/agents/capiko-sdd-design.agent.md` — references `sdd-design/SKILL.md`.
- [x] 1.8 Create `internal/catalog/agents/capiko-sdd-tasks.agent.md` — references `sdd-tasks/SKILL.md`.
- [x] 1.9 Create `internal/catalog/agents/capiko-sdd-apply.agent.md` — `tools: [read,edit,search,execute]`; references `sdd-apply/SKILL.md`.
- [x] 1.10 Create `internal/catalog/agents/capiko-sdd-verify.agent.md` — `tools: [read,edit,search,execute]`; references `sdd-verify/SKILL.md`.
- [x] 1.11 Create `internal/catalog/agents/capiko-sdd-archive.agent.md` — `tools: [read,edit,search]`; references `sdd-archive/SKILL.md`.
- [x] 1.12 Create `internal/catalog/agents/capiko-sdd-coordinator.agent.md` — `tools: [execute,read,agent]`; `user-invocable: true`; `agents:` allowlist with all 8 worker names; body: full language contract + deterministic routing algorithm using `capiko-ai sdd-status --json` + explicit `agent` tool delegation.

### Phase 1.5 — GREEN: Wire `catalog.LoadAgents`

- [x] 1.13 Modify `internal/catalog/catalog.go`:
  - Add `//go:embed agents` directive alongside existing `//go:embed skills`.
  - Add `func LoadAgents() ([]agent.Agent, error)` — `fs.Sub(files, "agents")` → `agent.LoadCatalog(sub)`.
- [x] 1.14 Add `internal/catalog/catalog_test.go` test `TestLoadAgents_ReturnsNine` — calls `catalog.LoadAgents()`, asserts 9 agents returned with no error. (Exercises the embed at test time, catching authoring errors.)

### Slice 1 Verification Boundary

- [x] 1.15 Run `go test -race ./internal/agent/... ./internal/catalog/...` — must exit 0.
- [x] 1.16 Run `gofmt -l .` — must produce empty output.
- [x] 1.17 Run `go vet ./...` — must produce no diagnostics.

---

## Slice 2 — Host + State + Drift: `copilot.Host`, `state.Agents`, `drift.StaleAgents`

> **Spec requirements covered**: AgentsDirOnHost, InstalledAgentsEnumeration, UninstallAgentIdempotencyAndSafety, DriftDetection  
> **Verification**: `go test -race ./internal/copilot/... ./internal/state/... ./internal/drift/...`

### Phase 2.1 — RED: Host + state + drift tests

- [ ] 2.1 Add to `internal/copilot/copilot_test.go`:
  - `TestDetect_AgentsDirDerivation` — assert `h.AgentsDir == filepath.Join(h.ConfigDir, "agents")` (spec: AgentsDirOnHost / Scenario: Host is detected normally).
  - `TestHost_InstalledAgents_MixedFiles` — tmp `agentsDir` with `sdd-explore.agent.md`, `sdd-apply.agent.md`, `README.md`; assert result contains `sdd-explore` and `sdd-apply`, not `README` (spec: InstalledAgentsEnumeration / Scenario: AgentsDir exists with mixed files).
  - `TestHost_InstalledAgents_MissingDir` — non-existent `agentsDir`; assert empty map + nil error (spec: InstalledAgentsEnumeration / Scenario: AgentsDir does not exist).
  - `TestHost_UninstallAgent_RemovesFile` — present file; assert removed + nil error (spec: UninstallAgentIdempotencyAndSafety / Scenario: Agent is present and is removed).
  - `TestHost_UninstallAgent_Idempotent` — missing file; assert nil error (spec: UninstallAgentIdempotencyAndSafety / Scenario: Agent is already absent).
  - `TestHost_UninstallAgent_RefusesNonAgentMd` — name resolving outside `.agent.md` pattern; assert non-nil error + no file removed (spec: UninstallAgentIdempotencyAndSafety / Scenario: Name resolves to a non-agent-md file).

- [ ] 2.2 Add to `internal/state/state_test.go`:
  - `TestState_AgentsMap_InitializedOnLoad` — empty state.json loads with non-nil `Agents` map.
  - `TestStore_ApplyAgents_RecordsChecksums` — `ApplyAgents` persists `AgentRecord{Checksum, InstalledAt}` for given agent names.

- [ ] 2.3 Add to `internal/drift/drift_test.go`:
  - `TestStaleAgents_AllInSync` — catalog + matching state → zero entries (spec: DriftDetection / Scenario: All agents in sync).
  - `TestStaleAgents_MissingAgent` — agent in catalog absent from state → `missing` drift item (spec: DriftDetection / Scenario: One agent is missing).
  - `TestStaleAgents_ChangedContent` — agent in state with stale checksum → `changed` drift item (spec: DriftDetection / Scenario: One agent has changed content).

### Phase 2.2 — GREEN: Implement `copilot.Host` agent methods

- [ ] 2.4 Modify `internal/copilot/copilot.go`:
  - Add `AgentsDir string` field to `Host` struct.
  - Set `AgentsDir: filepath.Join(cfg, "agents")` in `Detect()` return.
  - Add `func (h *Host) InstalledAgents() (map[string]bool, error)` — enumerate `*.agent.md` files (not dirs) in `AgentsDir`; return empty map + nil on `os.IsNotExist`.
  - Add `func (h *Host) UninstallAgent(name string) error` — compute target path `filepath.Join(h.AgentsDir, name+".agent.md")`; guard that resolved path ends in `.agent.md`; idempotent on missing; `os.Remove` on present.

### Phase 2.3 — GREEN: Implement `state.Agents` map + `ApplyAgents`

- [ ] 2.5 Modify `internal/state/state.go`:
  - Add `AgentRecord struct { Checksum string; InstalledAt time.Time }` (JSON tags matching `SkillRecord` pattern).
  - Add `Agents map[string]AgentRecord` field to `State` (json tag `"agents,omitempty"`).
  - In `Load()`: after unmarshaling, add `if st.Agents == nil { st.Agents = map[string]AgentRecord{} }` guard.
  - Add `func (s *Store) ApplyAgents(version string, installed []Installed, removed []string) error` — load, update `st.Agents`, stamp version + time, save.

### Phase 2.4 — GREEN: Implement `drift.StaleAgents`

- [ ] 2.6 Modify `internal/drift/drift.go`:
  - Add `func StaleAgents(catalog []agent.Agent, st *state.State) []string` — iterate catalog; for each agent, check `st.Agents[name]`; if absent → `missing`; if checksum mismatch → `changed`; collect stale names in catalog order.

### Slice 2 Verification Boundary

- [ ] 2.7 Run `go test -race ./internal/copilot/... ./internal/state/... ./internal/drift/...` — must exit 0.
- [ ] 2.8 Run `gofmt -l .` — must produce empty output.
- [ ] 2.9 Run `go vet ./...` — must produce no diagnostics.

---

## Slice 3 — TUI Wiring: `RunSync`, `app.go`, drift banner

> **Spec requirements covered**: TUISurfacesAgentsAlongsideSkills, TestSuitePassesUnderStrictTDD  
> **Verification**: `go test -race ./...`

### Phase 3.1 — RED: TUI tests

- [ ] 3.1 Add to `internal/tui/sync_test.go`:
  - `TestRunSync_InstallsAgents` — mock agent catalog; assert `AgentsDir` receives `<name>.agent.md` files and `ApplyAgents` is called (spec: TUISurfacesAgentsAlongsideSkills / Scenario: Install screen shows agents).
  - `TestRunSync_AgentCountReturned` — assert returned count includes agents written.

- [ ] 3.2 Add to `internal/tui/app_test.go`:
  - `TestApp_StaleBanner_IncludesAgents` — stale agent + stale skill in state; assert banner count covers both (spec: TUISurfacesAgentsAlongsideSkills / Scenario: Drift screen shows drifted agents).
  - `TestApp_DetectCmd_PopulatesInstalledAgents` — `detectedMsg` carries `installedAgents`; assert `a.installedAgents` set.

### Phase 3.2 — GREEN: Extend `RunSync` with agent install

- [ ] 3.3 Modify `internal/tui/sync.go` — `RunSync` signature gains `agentCatalog []agent.Agent`:
  - After skills install loop, iterate `agentCatalog`: call `a.Install(host.AgentsDir)`.
  - Collect `[]state.Installed` for agents; call `store.ApplyAgents(Version, agentRecorded, nil)` after skills `store.Apply`.
  - Return `len(recorded) + len(agentRecorded)` as total count.
  - Update `syncScreen.syncCmd` to pass agent catalog from `svc`.

### Phase 3.3 — GREEN: Extend `app.go` with agent awareness

- [ ] 3.4 Modify `internal/tui/app.go`:
  - Add `agentCatalog []agent.Agent` and `installedAgents map[string]bool` and `staleAgents []string` fields to `App`.
  - Update `NewApp` signature to accept `agentCatalog []agent.Agent`.
  - In `detectCmd`/`detectedMsg`: call `h.InstalledAgents()`; populate `a.installedAgents`.
  - In `Update` on `detectedMsg`: populate `a.staleAgents` via `drift.StaleAgents(a.agentCatalog, st)`.
  - Update `staleBanner` to include agent drift count alongside skill drift count (distinct "Agents" label per spec).
  - Pass `a.agentCatalog` to `newSync` and `newDetection` call sites.

### Phase 3.4 — GREEN: Update call sites in `main`

- [ ] 3.5 Update `main.go` (or wherever `NewApp` and `RunSync` are called): pass `catalog.LoadAgents()` result alongside skills catalog. Fail fast with error if agent catalog fails to load.

### Slice 3 Verification Boundary

- [ ] 3.6 Run `go test -race ./...` — must exit 0 (spec: TestSuitePassesUnderStrictTDD / Scenario: CI gate passes).
- [ ] 3.7 Run `gofmt -l .` — must produce empty output (spec: TestSuitePassesUnderStrictTDD / Scenario: Formatting gate passes).
- [ ] 3.8 Run `go vet ./...` — must produce no diagnostics.
