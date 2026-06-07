# copilot-sdd-agents Specification

## Purpose

This spec defines what MUST be true after `copilot-sdd-agents` is applied.
It covers the `copilot-sdd-agents` capability: install/sync/uninstall an embedded catalog of
SDD-phase Copilot custom agents (`.agent.md`) into `~/.copilot/agents/`, with drift detection
and a coordinator that routes via the native engine.

This is a new capability — no existing spec to delta against.

---

## Requirements

### Requirement: AgentsDir on Host

`copilot.Host` MUST expose an `AgentsDir` field (`~/.copilot/agents`) computed from `ConfigDir`,
mirroring how `SkillsDir` is derived.

#### Scenario: Host is detected normally

- GIVEN Copilot CLI is installed and `~/.copilot` exists
- WHEN `Detect()` returns a `*Host`
- THEN `h.AgentsDir` equals `filepath.Join(h.ConfigDir, "agents")`

---

### Requirement: InstalledAgents Enumeration

`Host.InstalledAgents()` MUST return the set of agent names already present in `AgentsDir` —
every file matching `*.agent.md` (by filename, not directory structure). A missing `AgentsDir`
MUST be treated as "none installed", not an error.

#### Scenario: AgentsDir exists with mixed files

- GIVEN `AgentsDir` contains `sdd-explore.agent.md`, `sdd-apply.agent.md`, and `README.md`
- WHEN `InstalledAgents()` is called
- THEN the result contains `"sdd-explore"` and `"sdd-apply"` and does NOT contain `"README"`

#### Scenario: AgentsDir does not exist

- GIVEN `AgentsDir` has never been created
- WHEN `InstalledAgents()` is called
- THEN it returns an empty map and a nil error

---

### Requirement: UninstallAgent Idempotency and Safety

`Host.UninstallAgent(name string)` MUST remove `<AgentsDir>/<name>.agent.md`. It MUST be
idempotent (file already absent is not an error). It MUST refuse to remove any file that is
not a `.agent.md` (guards against bad names deleting arbitrary files), returning an error.

#### Scenario: Agent is present and is removed

- GIVEN `<AgentsDir>/sdd-spec.agent.md` exists
- WHEN `UninstallAgent("sdd-spec")` is called
- THEN the file is removed and nil is returned

#### Scenario: Agent is already absent

- GIVEN `<AgentsDir>/sdd-spec.agent.md` does not exist
- WHEN `UninstallAgent("sdd-spec")` is called
- THEN nil is returned (idempotent)

#### Scenario: Name resolves to a non-agent-md file

- GIVEN a name whose resolved path does not end in `.agent.md` (e.g. via path traversal)
- WHEN `UninstallAgent` is called
- THEN it returns a non-nil error and removes nothing

---

### Requirement: Embedded Agent Catalog

The binary MUST embed a catalog of `.agent.md` files under `assets/agents/` (analogous to
`catalog/skills/`). The catalog MUST contain exactly one file per SDD phase
(`sdd-explore`, `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`,
`sdd-verify`, `sdd-archive`) plus one coordinator (`sdd-coordinator`). Each embedded file
MUST parse without error at build time (caught by tests, not at runtime).

#### Scenario: Catalog is loaded

- GIVEN the embedded `assets/agents/` FS
- WHEN the agent catalog loader is called
- THEN it returns exactly 9 `Agent` values (8 workers + 1 coordinator) with no error

#### Scenario: Malformed embedded agent file

- GIVEN an embedded `.agent.md` with invalid or missing frontmatter
- WHEN the catalog is loaded
- THEN `LoadAgentCatalog` returns a non-nil error (authoring error caught at test time)

---

### Requirement: Agent Install and Sync

Installing an agent from the catalog MUST write `<name>.agent.md` under `AgentsDir`,
creating `AgentsDir` if it does not exist. Re-installing when the installed content matches
the catalog content (checksum equal) MUST be a no-op (zero writes). Re-installing when
content differs MUST overwrite.

#### Scenario: First install

- GIVEN `AgentsDir` does not exist
- WHEN the agent catalog is installed
- THEN `AgentsDir` is created and one `.agent.md` file per catalog entry exists within it

#### Scenario: Re-install with identical content (idempotency)

- GIVEN all agents are already installed with content matching the catalog
- WHEN install is called again
- THEN no files are written (observable via filesystem mtime or write-call mock)

#### Scenario: Re-install after content drift

- GIVEN an installed `.agent.md` whose content differs from the catalog copy
- WHEN install is called
- THEN the file is overwritten with catalog content

---

### Requirement: Drift Detection

The drift checker MUST report any agent whose installed `.agent.md` is missing or whose
content differs from the embedded catalog. It MUST use the same checksum strategy as the
skills drift checker. A fully in-sync installation MUST produce zero drift entries.

#### Scenario: All agents in sync

- GIVEN all catalog agents are installed with matching content
- WHEN drift is checked
- THEN zero drift items are reported

#### Scenario: One agent is missing

- GIVEN `sdd-spec.agent.md` is absent from `AgentsDir`
- WHEN drift is checked
- THEN a drift item for `sdd-spec` with status `missing` is reported

#### Scenario: One agent has changed content

- GIVEN `sdd-apply.agent.md` content differs from the catalog copy
- WHEN drift is checked
- THEN a drift item for `sdd-apply` with status `changed` is reported

---

### Requirement: Worker Agent Shape

Each worker `.agent.md` MUST satisfy all of the following structural constraints:

1. Frontmatter field `user-invocable: false` MUST be present.
2. Frontmatter field `tools` MUST list only Copilot-native aliases
   (`read`, `edit`, `search`, `execute`, `agent`). It MUST NOT include any Anthropic model
   aliases or invalid Copilot tool names.
3. Frontmatter MUST NOT include a `model:` field with an Anthropic alias
   (e.g. `opus`, `sonnet`, `haiku`). The field is either absent (to inherit the default)
   or set to a valid Copilot model name.
4. The body MUST reference the corresponding skill path
   (`~/.copilot/skills/<phase>/SKILL.md`) and MUST NOT copy phase logic inline.
5. The body MUST NOT instruct the agent to ignore or override model safety guardrails.

#### Scenario: Worker frontmatter validation

- GIVEN the embedded `sdd-explore.agent.md`
- WHEN its frontmatter is parsed
- THEN `user-invocable` is `false`, `tools` contains only allowed aliases, and no Anthropic model alias appears in `model:`

#### Scenario: Worker body references skill path

- GIVEN the embedded `sdd-spec.agent.md`
- WHEN its body is read
- THEN it contains the string `~/.copilot/skills/sdd-spec/SKILL.md`

---

### Requirement: Coordinator Routing via Native Engine

The coordinator agent's body MUST instruct it to determine the next SDD phase by invoking
`capiko-ai sdd-status --json` and reading `nextRecommended` from the JSON output. It MUST
then delegate EXPLICITLY to the corresponding phase worker via the `agent` tool and an
`agents:` allowlist — NOT via description-based inference. The `agents:` allowlist in the
coordinator frontmatter MUST name all eight phase workers.

#### Scenario: Coordinator frontmatter allowlist

- GIVEN the embedded `sdd-coordinator.agent.md`
- WHEN its frontmatter is parsed
- THEN the `agents:` list contains exactly the eight phase worker names

#### Scenario: Coordinator body cites native engine

- GIVEN the embedded `sdd-coordinator.agent.md`
- WHEN its body is read
- THEN it contains the string `capiko-ai sdd-status --json` and the string `nextRecommended`

#### Scenario: Coordinator body mandates explicit delegation

- GIVEN the embedded `sdd-coordinator.agent.md`
- WHEN its body is read
- THEN it explicitly references the `agent` tool for delegation (not description inference)

---

### Requirement: Language Contract in Agents

Each agent's body MUST carry an explicit language contract stating:
(a) replies to the human are in the human's language, and
(b) all SDD artifacts, handoffs, and envelopes are written in English.
The contract MUST appear in the coordinator and SHOULD appear in each worker.

#### Scenario: Coordinator carries language contract

- GIVEN the embedded `sdd-coordinator.agent.md`
- WHEN its body is read
- THEN it contains both "human's language" (or equivalent) and "English" for artifacts

#### Scenario: Worker carries language contract

- GIVEN any embedded worker agent (e.g. `sdd-apply.agent.md`)
- WHEN its body is read
- THEN it contains a language contract covering reply language and artifact language

---

### Requirement: TUI Surfaces Agents alongside Skills

The TUI MUST surface agent install and drift status in the same flow as skills, with
parity to how skills are shown. Installed agents MUST appear in a distinct labeled
"Agents" section, and agent drift MUST be surfaced the same way skill drift is — via the
menu's "out of date" banner, which counts drifted agents alongside drifted skills. (capiko
has no per-item drift screen for skills either; the banner is the established drift
surface.)

#### Scenario: Sync view shows an Agents section

- GIVEN the TUI sync flow completes
- WHEN the done view renders
- THEN it includes a labeled "Agents" section listing the synced agent names, parallel to the "Skills" section

#### Scenario: Drift banner counts drifted agents

- GIVEN one or more agents have drifted (missing from state or content-changed)
- WHEN the menu renders
- THEN the "out of date" banner counts the drifted agents alongside any drifted skills, pointing the user at Sync configs

---

### Requirement: Test Suite Passes Under Strict TDD

All new and modified packages MUST pass `go test -race ./...` with zero failures. No new
code path MUST be introduced without a corresponding test. `gofmt -l .` MUST produce empty
output. `go vet ./...` MUST produce no diagnostics.

#### Scenario: CI gate passes

- GIVEN all changes from this capability are applied
- WHEN `go test -race ./...` is executed
- THEN the exit code is 0 and no race conditions are reported

#### Scenario: Formatting gate passes

- GIVEN all new `.go` files
- WHEN `gofmt -l .` is executed
- THEN the output is empty (no unformatted files)
