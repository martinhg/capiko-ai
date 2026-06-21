# Capabilities

Everything capiko-ai can do today, grouped by what you're trying to accomplish. If
you're new, start with the [README](../README.md) and [usage.md](usage.md); this page
is the full tour.

The throughline: **capiko configures, Copilot executes.** capiko never calls a model.
Every capability below is either a file it writes for Copilot to discover, or a
deterministic engine it runs in Go.

## Configure Copilot

| Capability | What it does |
|---|---|
| **Skill catalog** | Installs a curated set of `SKILL.md` files into `~/.copilot/skills/`, which Copilot auto-discovers. Declarative checkbox reconcile — check to install, uncheck to remove. |
| **Personas** | Writes a marker-bound instruction block into `~/.copilot/copilot-instructions.md`: **Capiko** (teaching-first, Rioplatense), **Neutral** (same guidance, professional tone), or **None**. Tracked in state, re-applied on sync. |
| **Scoped instructions** | Installs curated `*.instructions.md` files (with `applyTo` globs) into `~/.copilot/instructions/` and every `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` entry. Copilot applies them per matching file. Opt-in: re-applied on sync only once you've installed them. |
| **Custom agents** | Installs `.agent.md` SDD-phase agents into `~/.copilot/agents/` — the real delegation targets the SDD workflow routes to. |
| **Code review (gga)** | Wires [Gentleman Guardian Angel](https://github.com/Gentleman-Programming/gentleman-guardian-angel) into a project: writes `.gga`, injects a curated `AGENTS.md` rules block (kept in sync with the active persona), and installs the pre-commit review hook. Pick the provider; gga must be on PATH. capiko configures it — it never runs the reviewer. |
| **Review and Confirm** | A pre-apply summary (skills to install/remove, active persona) gates every reconcile, so nothing is written until you've seen it. |

## Headless & scriptable

Every install operation also runs non-interactively, so capiko drops straight into CI,
dotfiles bootstrap, and automation.

| Capability | What it does |
|---|---|
| **`install` / `sync` / `uninstall`** | The full reconcile surface without the TUI. `install` is additive (never touches what's present); `sync` overwrites to match the catalog (`--auto-repair` only writes when drift is detected); `uninstall` removes managed items and **refuses** when the state store is unavailable rather than risk deleting unmanaged files. |
| **`--json` everywhere** | Machine-readable output on every command for scripting and assertions. |
| **`--verbose` diagnostics** | Streams structured JSON-lines events (timestamp, event, result, duration) to **stderr** on every action command, so stdout stays clean for scripting. |
| **Deterministic exit codes** | `0` success · `1` error · `2` Copilot CLI not found — usable directly in pipelines. |
| **No TTY required** | The command path never launches Bubbletea, so it runs headless in CI. |

See [usage.md](usage.md#headless-cli) for the full command table.

## Spec-Driven Development

The headline workflow. Ask Copilot to make a substantial change and it runs eight
phases instead of free-styling: `explore → propose → spec → design → tasks → apply →
verify → archive`.

| Capability | What it does |
|---|---|
| **Native SDD engine** | `capiko-ai sdd-status` / `sdd-continue` compute the workflow state deterministically in Go from the OpenSpec store. The skills prefer the native command and fall back to inference only when the binary is unavailable. See [native-sdd-engine.md](native-sdd-engine.md). |
| **SDD Status dashboard** | A read-only TUI screen lists active OpenSpec changes with each change's next phase, task progress, and a per-change phase graph (done / in progress / blocked) plus any blocked reasons — the same state the native engine computes, rendered interactively. |
| **Per-phase model routing** | A "Configure SDD" screen assigns a model to each phase; the orchestrator delegates each phase to its model — architecture phases on a top model, mechanical phases auto-downgraded. |
| **Phase skills + agents** | Each phase ships as a catalog skill (`sdd-explore`, `sdd-propose`, …) with a matching `.agent.md` delegation target and a `## Gate` that forces delegation rather than inline execution. |
| **Strict TDD forwarding** | A `t` toggle that, when on, forwards the failing-test-first protocol and the test command into the apply/verify handoff — not just a one-time mention. |
| **Review-workload guard** | `sdd-tasks` forecasts the change size and a >400-line PR budget before apply, steering oversized changes into reviewable chained PRs. |
| **Delivery & chain strategy** | A decision flow that caches a delivery strategy (`ask-on-risk \| auto-chain \| single-pr \| exception-ok`) and a chain strategy (`stacked-to-main \| feature-branch-chain`) when the forecast chains. |
| **SDD triage rules** | Both the orchestrator block and the coordinator agent carry the same rules: inline for small changes, delegate on the 4-file rule, full SDD only for substantial work, fresh review before a non-trivial PR — so token cost matches change size. |
| **`sdd-init` / `sdd-onboard`** | `sdd-init` bootstraps the OpenSpec store; `sdd-onboard` is a guided walkthrough that teaches the cycle on your real code. |
| **Skill-registry resolution** | `capiko-ai skill-registry` scans installed skills (user + project scope) and prints them indexed by trigger, scope, and absolute path — always fresh — so delegators inject the exact paths into sub-agents. |
| **Skill dependency graph** | `depends_on` frontmatter declares which skills a skill needs; the catalog validates the graph (no missing or cyclic deps) so a phase skill never ships expecting a sibling that isn't there. |

## Cross-session memory (engram)

| Capability | What it does |
|---|---|
| **MCP wiring** | Detects [engram](https://github.com/Gentleman-Programming/engram) and registers its MCP server into Copilot CLI (`~/.copilot/mcp-config.json`) **and** VS Code (`.vscode/mcp.json` or the user-level `mcp.json`) — merging, never clobbering other servers. |
| **Artifact-store modes** | Pick `hybrid` (both — the team default), `engram` (memory only), `openspec` (files only), or `none`. |
| **Engram Cloud** | Configures the cloud client, enrolls the project, and writes a per-repo `.engram/config.json` so memories are scoped correctly in multi-repo workspaces. The token is never written to disk — only the `${ENGRAM_CLOUD_TOKEN}` reference. |
| **Team server scaffold** | Ships a hardened `docker-compose.cloud.yml` and a [setup guide](engram-cloud-setup.md) for devops to stand up the shared server. |

Tracked in `state.json`, re-applied on sync, surfaced as drift. capiko configures the
client and never runs the infra.

## Token & context efficiency

| Capability | What it does |
|---|---|
| **Output-efficiency block** | An opt-in (off by default) instruction block that trims ceremony and restated context to cut output tokens — while keeping full rigor on new questions and errors. Injected, tracked in state, and re-applied on sync like the persona. |
| **Context compression (headroom)** | Opt-in. Wires the [headroom](https://github.com/chopratejas/headroom) (Apache-2.0) MCP server into Copilot for context compression and instructs the agent to use it — fewer tokens, same answers. capiko configures it; it never installs or runs the tool. Drift-tracked, with a doctor warning when the configured binary is stale. |

## Environment & safety

| Capability | What it does |
|---|---|
| **System Detection** | Reports OS/shell, the tools capiko relies on, dependency versions, and which Copilot/engram configs exist. |
| **Health check** | `capiko-ai doctor` runs a read-only diagnosis — OS support, required prerequisites (copilot/node/npm/pnpm/git/curl), Copilot init, `state.json` validity, skill/agent drift, and the engram backend — printing `pass`/`warn`/`fail` per check with a remedy, or `--json` for tooling. Exits non-zero when any check fails. |
| **One-click dependency install** | Shows a per-distro install command for each missing dependency (apt/pacman/dnf/winget/brew, detected from `/etc/os-release`). **Install missing** runs the safe, no-sudo ones; sudo commands are shown but never auto-run. |
| **Backups** | Snapshot-before-mutate: every change copies the affected skills, agent files, and standalone files to `~/.capiko/backups/<id>/` with a manifest first — install and uninstall snapshot skills and agents together, so a restore is symmetric. Browse, restore, or delete them from **Manage backups**. |
| **Persistent state** | `~/.capiko/state.json` (atomic writes) records every installed skill with a content checksum, plus the persona, SDD config, scoped-instruction flag, and engram config. |
| **Drift detection** | Pure catalog-vs-state checksum comparison flags which managed items have gone stale. |
| **Self-update** | A banner detects new releases; **Upgrade tools** updates in place (brew/go/binary) and restarts. **Upgrade + sync** also re-applies the new catalog. |

## Documentation generation

| Capability | What it does |
|---|---|
| **`skill-creator`** | A catalog skill that guides Copilot to scaffold a new custom `SKILL.md` from a plain-language description. |
| **`codebase-docs`** | A catalog skill that guides Copilot to generate a `docs/codebase/` guide (mental model, repository map, architecture) for **your** project, so new devs onboard fast. |

## Release & distribution

A goreleaser pipeline ships capiko on every `v*` tag: multi-platform binaries with
checksums, a Homebrew cask, a Scoop manifest, and install scripts for macOS/Linux and
Windows. See [release.md](release.md).

## Roadmap

- **Copilot CLI model selection** — surfaced in the SDD config screen if/when the
  Copilot CLI exposes a model picker.
