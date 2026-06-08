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
| **Review and Confirm** | A pre-apply summary (skills to install/remove, active persona) gates every reconcile, so nothing is written until you've seen it. |

## Spec-Driven Development

The headline workflow. Ask Copilot to make a substantial change and it runs eight
phases instead of free-styling: `explore → propose → spec → design → tasks → apply →
verify → archive`.

| Capability | What it does |
|---|---|
| **Native SDD engine** | `capiko-ai sdd-status` / `sdd-continue` compute the workflow state deterministically in Go from the OpenSpec store. The skills prefer the native command and fall back to inference only when the binary is unavailable. See [native-sdd-engine.md](native-sdd-engine.md). |
| **Per-phase model routing** | A "Configure SDD" screen assigns a model to each phase; the orchestrator delegates each phase to its model — architecture phases on a top model, mechanical phases auto-downgraded. |
| **Phase skills + agents** | Each phase ships as a catalog skill (`sdd-explore`, `sdd-propose`, …) with a matching `.agent.md` delegation target and a `## Gate` that forces delegation rather than inline execution. |
| **Strict TDD forwarding** | A `t` toggle that, when on, forwards the failing-test-first protocol and the test command into the apply/verify handoff — not just a one-time mention. |
| **Review-workload guard** | `sdd-tasks` forecasts the change size and a >400-line PR budget before apply, steering oversized changes into reviewable chained PRs. |
| **Delivery & chain strategy** | A decision flow that caches a delivery strategy (`ask-on-risk \| auto-chain \| single-pr \| exception-ok`) and a chain strategy (`stacked-to-main \| feature-branch-chain`) when the forecast chains. |
| **SDD triage rules** | Both the orchestrator block and the coordinator agent carry the same rules: inline for small changes, delegate on the 4-file rule, full SDD only for substantial work, fresh review before a non-trivial PR — so token cost matches change size. |
| **`sdd-init` / `sdd-onboard`** | `sdd-init` bootstraps the OpenSpec store; `sdd-onboard` is a guided walkthrough that teaches the cycle on your real code. |
| **Skill-registry resolution** | `capiko-ai skill-registry` scans installed skills (user + project scope) and prints them indexed by trigger, scope, and absolute path — always fresh — so delegators inject the exact paths into sub-agents. |

## Cross-session memory (engram)

| Capability | What it does |
|---|---|
| **MCP wiring** | Detects [engram](https://github.com/Gentleman-Programming/engram) and registers its MCP server into Copilot CLI (`~/.copilot/mcp-config.json`) **and** VS Code (`.vscode/mcp.json` or the user-level `mcp.json`) — merging, never clobbering other servers. |
| **Artifact-store modes** | Pick `hybrid` (both — the team default), `engram` (memory only), `openspec` (files only), or `none`. |
| **Engram Cloud** | Configures the cloud client, enrolls the project, and writes a per-repo `.engram/config.json` so memories are scoped correctly in multi-repo workspaces. The token is never written to disk — only the `${ENGRAM_CLOUD_TOKEN}` reference. |
| **Team server scaffold** | Ships a hardened `docker-compose.cloud.yml` and a [setup guide](engram-cloud-setup.md) for devops to stand up the shared server. |

Tracked in `state.json`, re-applied on sync, surfaced as drift. capiko configures the
client and never runs the infra.

## Environment & safety

| Capability | What it does |
|---|---|
| **System Detection** | Reports OS/shell, the tools capiko relies on, dependency versions, and which Copilot/engram configs exist. |
| **One-click dependency install** | Shows a per-distro install command for each missing dependency (apt/pacman/dnf/winget/brew, detected from `/etc/os-release`). **Install missing** runs the safe, no-sudo ones; sudo commands are shown but never auto-run. |
| **Backups** | Snapshot-before-mutate: every change copies the affected skills and standalone files to `~/.capiko/backups/<id>/` with a manifest first. Browse, restore, or delete them from **Manage backups**. |
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
