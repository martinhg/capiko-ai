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
- SDD init / onboard — `sdd-init` bootstraps per-project context (`sdd/context.md`)
  so phases don't re-discover the project; `sdd-onboard` is a guided walkthrough
  that teaches the SDD cycle on the user's real code.
- Strict TDD toggle — a `t` toggle on the Configure SDD screen; when on, the
  orchestrator block requires the apply/verify phases to follow strict TDD
  (failing test first). Tracked in state and re-applied on sync.
- Dependency install hints + one-click install — System Detection shows a
  per-OS install command for each missing dependency, and "Install missing" runs
  the safe (non-sudo) ones. Also detects `pnpm` (company requirement) with version.
- Scoped instructions — curated `*.instructions.md` files (with `applyTo` globs)
  installed under `~/.copilot/instructions/`, which Copilot applies per matching
  file. "Install instructions" menu item writes them (with backup); detection
  reports the directory.

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
- **Agent Builder → "skill-creator" (decided approach).** gentle-ai's Agent
  Builder is an LLM-generation wizard (describe → generate → preview → install):
  it *calls a model* to write a custom agent. capiko has no LLM in its Go code (it
  is a file configurator), and adding an API client/auth/cost would break that
  pattern. The capiko-appropriate version keeps the LLM where it belongs — Copilot
  — by shipping a `skill-creator` **catalog skill** that guides Copilot to scaffold
  a new custom `SKILL.md` from the user's description. capiko ships the guidance;
  Copilot does the building. (Same pattern as the SDD skills.)
- **SDD skills are deliberately simple (TODO: full machinery).** capiko's
  `sdd-*` skills are self-contained and file-based on purpose, to ship the workflow
  fast. gentle-ai's SDD skills are a richer machine: an artifact-store layer
  (engram / openspec / hybrid persistence), `_shared` status contracts passed
  between phases, orchestrator/executor gates, delivery-strategy + workload guards,
  strict-TDD forwarding, and per-skill registries. A later pass should evolve
  capiko's skills toward that — add a persistence backend (engram/openspec), a
  shared status contract the phases read/write, and the orchestrator gates — so the
  cycle is resumable and cross-session, not just a set of guidance docs.
- **One-click install on Linux** — install hints exist for all platforms, but the
  one-click runner only auto-runs no-sudo commands (brew on macOS, the pnpm
  installer). Linux system packages (git/curl via apt) and node/go are shown but
  not auto-run, partly because sudo can't prompt inside the TUI. A safe Linux
  one-click (distro detection + a non-TUI sudo path) is future work.
- **`COPILOT_CUSTOM_INSTRUCTIONS_DIRS` support** — Copilot also honors extra
  instruction dirs via this env var; capiko manages the home file and the
  `~/.copilot/instructions/` dir, but not arbitrary configured dirs.

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
