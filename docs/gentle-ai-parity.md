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
- **Agent Builder** — gentle-ai can scaffold a custom agent (prompt → preview →
  generate → install). A "build a custom Copilot skill" wizard would be the
  capiko analogue.
- **SDD integration / profiles** — gentle-ai installs an SDD orchestrator and
  manages OpenCode SDD profiles. capiko could ship an SDD skill bundle for Copilot.
- **Strict TDD mode** toggle.
- **Review screen** — a pre-apply diff/summary of everything that will change.
- **Dependency install hints / one-click install** — gentle-ai offers install
  hints (and can run them) for missing dependencies; our detection only reports.
- **`copilot-instructions` directory support** — Copilot also reads
  `~/.copilot/instructions/` and `COPILOT_CUSTOM_INSTRUCTIONS_DIRS`; capiko only
  manages the single home instructions file today.

## Notes

- gentle-ai uses the Rose Pine palette; capiko keeps its warm amber + capybara brand.
- gentle-ai is a much larger, more mature codebase (multi-agent, agent builder,
  profiles). capiko is focused: it does the core configurator flow well for Copilot.
