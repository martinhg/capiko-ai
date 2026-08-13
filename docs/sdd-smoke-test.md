# SDD Pipeline Smoke Test

Manual validation that capiko-generated configs produce a working SDD pipeline
in GitHub Copilot CLI. Run this after any change to `internal/catalog/skills/sdd-*`,
`internal/catalog/agents/capiko-sdd-*`, `internal/sdd/`, or `internal/tui/` install/sync
paths.

## Prerequisites

- `capiko-ai` built from the branch under test (`go build ./cmd/capiko-ai`)
- GitHub Copilot CLI installed and authenticated (`copilot --version`)
- A test repository with at least one source file (any language)

## 1. Install and verify SDD configs

```sh
capiko-ai install --all --json
```

**Check**:
- `~/.copilot/skills/sdd-*/SKILL.md` — all 11 SDD skills present
- `~/.copilot/agents/capiko-sdd-*.agent.md` — all 12 agents present (8 workers + coordinator + 3 JD)
- `~/.copilot/copilot-instructions.md` contains `<!-- capiko:sdd:start -->` / `<!-- capiko:sdd:end -->` block
- `~/.capiko/state.json` has non-empty `sdd_models` map

If `sdd_models` is empty or the SDD block is missing, the headless seeding (PR #180) is broken.

## 2. Verify sync re-applies

```sh
capiko-ai sync --json
```

**Check**: SDD block unchanged (diff-gated, idempotent). No errors in output.

## 3. Run a minimal SDD cycle

Open Copilot CLI in the test repo and invoke the coordinator:

```
@capiko-sdd-coordinator Create a small SDD change: add a hello-world endpoint
```

### 3.1 Propose phase

**Check**:
- [ ] Agent asks 3-5 proposal questions (Step 0) before writing the proposal
- [ ] `openspec/changes/<name>/proposal.md` exists on disk (not phantom)
- [ ] If non-interactive: proposal contains a `## Proposal Question Round` section

**Regression**: if the file is claimed but doesn't exist, the § D verify-after-write
guardrail is not being followed.

### 3.2 Spec phase

**Check**:
- [ ] `openspec/changes/<name>/spec.md` exists on disk
- [ ] Contains requirements with GIVEN/WHEN/THEN scenarios

### 3.3 Design phase

**Check**:
- [ ] `openspec/changes/<name>/design.md` exists on disk
- [ ] Does NOT block on format validation of example data (UUIDs, timestamps)

**Regression**: if the agent rejects valid UUIDs in test fixtures, the § H Example
Data guard is not being followed.

### 3.4 Tasks phase

**Check**:
- [ ] `openspec/changes/<name>/tasks.md` exists on disk
- [ ] Contains a `Review Workload Forecast` section
- [ ] Contains `Decision needed before apply:`, `Chained PRs recommended:`, `400-line budget risk:`

### 3.5 Apply phase

**Check**:
- [ ] Code changes match the tasks
- [ ] If strict TDD is active: tests written before implementation

### 3.6 Verify phase

**Check**:
- [ ] Verify report contains CRITICAL/WARNING/SUGGESTION sections
- [ ] Agent runs tests itself (does not trust claims)

### 3.7 Archive phase

**Check**:
- [ ] `openspec/changes/archive/` directory created if it didn't exist
- [ ] Change folder moved to `openspec/changes/archive/<date>-<name>/`
- [ ] `openspec/specs/` updated with merged requirements

**Regression**: if archive fails with "Parent directory does not exist", the
explicit mkdir instruction is not being followed.

## 4. Coordinator routing

**Check**:
- [ ] Coordinator uses `capiko-ai sdd-status --json` for deterministic routing
- [ ] Does not re-infer phase order manually
- [ ] Delegates via the `agent` tool, not description-based inference

## 5. Known failure patterns

| Pattern | Root cause | What to check |
|---------|-----------|---------------|
| Agent claims file written but it doesn't exist | Missing verify-after-write | § D in sdd-phase-common.md |
| Design/verify blocks on valid UUIDs | Over-zealous format validation | § H in sdd-phase-common.md |
| Archive fails with missing directory | No mkdir before move | Step 4 in sdd-archive/SKILL.md |
| Skills installed but Copilot ignores them | Missing orchestrator block | SDD markers in copilot-instructions.md |
| Coordinator doesn't know about JD agents | Allowlist incomplete | `agents:` field in coordinator frontmatter |

## 6. Cleanup

```sh
capiko-ai uninstall --all --json
rm -rf openspec/
```
