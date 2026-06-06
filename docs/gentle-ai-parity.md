# gentle-ai parity

Where capiko-ai stands against [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai),
the multi-agent configurator it is modelled on. capiko targets **only the GitHub
Copilot CLI**, so some gentle-ai features are intentionally out of scope.

## Done

- Main menu (framed box, braille mascot, update badge)
- System Detection screen (OS/shell, tools, dependency versions, Copilot configs)
- Choose your Persona (Capiko / Neutral / None) → global `~/.copilot/copilot-instructions.md`
- Install / Managed uninstall (declarative reconcile)
- Sync configs (overwrite to match catalog) — **re-applies the active persona**
- Backups (snapshot/restore skills **and** standalone files like the instructions file)
- Self-update (banner + Upgrade tools) and Upgrade + sync
- Skill drift detection (catalog vs `state.json`)
- Release pipeline (Homebrew, Scoop, install scripts, multi-platform binaries)
- Persona lifecycle: persona is tracked in `state.json`, re-applied on sync, and
  its instructions file is backed up through the managed backup system.
- Review and Confirm screen — a pre-apply summary (skills to install/remove,
  active persona) gating the reconcile, between the skill selector and the apply.
- SDD orchestrator + per-phase model table — a "Configure SDD models" screen
  (curated list + custom) writes an orchestrator instructions block that delegates
  each SDD phase to its assigned model via the Task tool. Tracked in state and
  re-applied on sync.
- SDD phase skills bundle — `sdd-explore/propose/spec/design/tasks/apply/verify/
  archive` ship in the catalog; the orchestrator delegates each phase to its skill.
- SDD init / onboard — `sdd-init` bootstraps the OpenSpec store; `sdd-onboard` is a
  guided walkthrough that teaches the SDD cycle on the user's real code.
- OpenSpec artifact store — the SDD skills now use a formal file-based store:
  `openspec/config.yaml` (project context), `openspec/changes/<name>/` (in-flight
  proposal/spec/design/tasks), `openspec/specs/` (canonical specs), and
  `openspec/changes/archive/`. Archive merges the change's spec delta into the
  canonical specs, making the cycle resumable and auditable.
- Strict TDD toggle — a `t` toggle on the Configure SDD screen; when on, the
  orchestrator block requires the apply/verify phases to follow strict TDD
  (failing test first). Tracked in state and re-applied on sync.
- Dependency install hints + one-click install — System Detection shows a
  per-OS install command for each missing dependency, and "Install missing" runs
  the safe (non-sudo) ones. Also detects `pnpm` (company requirement) with version.
- Scoped instructions — curated `*.instructions.md` files (with `applyTo` globs)
  installed under `~/.copilot/instructions/`, which Copilot applies per matching
  file. "Install instructions" menu item writes them (with backup); detection
  reports the directory and any `COPILOT_CUSTOM_INSTRUCTIONS_DIRS`.
- `skill-creator` skill — guides Copilot to scaffold a new custom `SKILL.md` from a
  plain-language description (the capiko analogue of gentle-ai's Agent Builder,
  without an LLM in capiko's Go).

## Intentionally out of scope (Copilot-only)

- **Select your Agent** — capiko always configures Copilot, so there is no agent
  picker. gentle-ai supports Claude Code, Codex, Gemini, OpenCode, etc.
- Per-agent persona overlays (Claude output-style, OpenCode `agent` block) — Copilot
  uses a single instructions file, which we already manage.
- `gentleman-neutral-artifacts` persona variant.
- Custom persona slot.

## Not yet implemented (candidate features)

Ordered roughly by value for a Copilot-focused tool:

- **Model configuration** — gentle-ai has per-agent model pickers. Copilot CLI
  model selection could be surfaced here if/when it exposes one.
- **More SDD machinery (TODO).** The OpenSpec file store is in place (config /
  changes / specs / archive + merge-on-archive). The pieces still missing vs
  gentle-ai, to evolve the SDD from convention-driven to machine-coordinated:
  - **engram backend + `hybrid` mode** — a cross-session memory DB (engram) as an
    alternative/companion artifact store, so the cycle's state survives across
    sessions and machines without relying on the repo's files. `hybrid` = files +
    engram together. gentle-ai selects `engram | openspec | hybrid | none` per change.
  - ~~**`_shared` status contracts**~~ — **Done.** Shipped as the `sdd-shared`
    multi-file skill bundle (`sdd-status-contract.md` + `sdd-phase-common.md`),
    enabled by the multi-file skill installer. A structured status object
    (`schemaName: capiko.sdd-status`: change root, artifact paths, apply/task
    progress, dependency states, action context) replaces re-reading loose
    markdown between phases.
  - ~~**Orchestrator/executor gates**~~ — **Done.** Every SDD phase skill carries
    a `## Gate`: if the orchestrator loaded the skill it must DELEGATE (not run it
    inline) and route via the status contract; the executor sub-agent loads
    `sdd-phase-common.md` and runs the phase body without re-delegating.
  - **Delivery-strategy + workload guards** — before apply, forecast the change
    size and decide PR strategy (`ask-on-risk | auto-chain | single-pr |
    exception-ok`) and chain strategy (stacked-to-main | feature-branch-chain),
    with a >400-line budget guard. Keeps large changes reviewable.
  - **Strict-TDD forwarding & skill registry** — forward the strict-TDD flag
    structurally to apply/verify sub-agents (we have the toggle, not the structural
    forwarding); and a per-skill registry indexing skills by trigger/path for the
    orchestrator to resolve.
- **One-click install on Linux** — install hints are now distro-aware: capiko
  detects the package manager from `/etc/os-release` (Ubuntu/Debian→apt, Arch→
  pacman, Fedora/RHEL→dnf, plus winget on Windows and Linuxbrew when present) and
  shows the correct per-distro command, mirroring gentle-ai's `install_deps.go`.
  Like gentle-ai, sudo system-package installs are shown but **not** auto-run; only
  no-sudo commands (Homebrew installs, the pnpm script) are one-click. gentle-ai
  itself never auto-runs sudo (it displays per-distro commands), so a TUI sudo
  handoff is not required for parity.
- **Manage instructions in `COPILOT_CUSTOM_INSTRUCTIONS_DIRS`** — System Detection
  now *reports* those configured dirs, but capiko only *writes* the home file and
  `~/.copilot/instructions/`. Writing/managing scoped files into the env-configured
  dirs is a possible next step.

## Documentation

- `docs/` is organized in three layers like gentle-ai's: **user** (`usage.md` +
  README), **maintainer** (`release.md`, this parity doc), and a **codebase guide**
  for contributors (`docs/codebase/`: `mental-model.md`, `repository-map.md`,
  `architecture.md`).
- A `codebase-docs` catalog skill ships the same idea to users: it guides Copilot to
  generate a `docs/codebase/` guide for **their** project, so new devs onboard fast.

## Dogfooding

- `AGENTS.md` + `skills/` at the repo root hold the conventions for **developing
  capiko** (workflow, Go/Bubbletea testing, commit discipline, branch-first PRs).
  Copilot loads `AGENTS.md` as custom instructions when working in this repo —
  mirroring gentle-ai's root `skills/`. Distinct from `internal/catalog/skills/`,
  which is the catalog shipped to users.

## Notes

- gentle-ai uses the Rose Pine palette; capiko keeps its warm amber + capybara brand.
- gentle-ai is a much larger, more mature codebase (multi-agent, agent builder,
  profiles). capiko is focused: it does the core configurator flow well for Copilot.
