# Usage

How to drive capiko-ai. For install/build, see the [README](../README.md).

## The install flow

`Start installation` walks you through, in order:

1. **System Detection** — your OS/shell, the tools capiko relies on, your
   dependency versions, and which Copilot configs exist. Missing dependencies show
   an install command; **Install missing** runs the safe ones.
2. **Choose your Persona** — `Capiko` (teaching-first, Rioplatense), `Neutral`
   (same guidance, professional tone), or `None`. Writes a marker-bound block into
   `~/.copilot/copilot-instructions.md`.
3. **Configure SDD models** — assign a model per SDD phase (←/→ to cycle, `c` for a
   custom id), and toggle **strict TDD** (`t`). The orchestrator runs on the top
   model; cheaper phases auto-downgrade.
4. **Pick skills** → **Review and Confirm** → apply. Review shows what will install
   and remove before anything is written.

**Configure SDD** and **Install instructions** are also their own menu items, so you
can change them later without re-running the whole flow. The persona is set inside
**Start installation**; re-run it (or **Sync configs**, which re-applies the active
persona) to change it.

## The other menu items

| Item | What it does |
|---|---|
| Managed uninstall | List installed capiko skills; unmark to remove. |
| Sync configs | Overwrite all installed skills to match the catalog; re-applies the persona and SDD blocks. |
| Manage backups | Browse / restore / delete the snapshots taken before each change. |
| Configure SDD | Edit the per-phase model table and strict-TDD toggle. |
| SDD Status | Read-only dashboard of active OpenSpec changes: next phase, task progress, and a per-change phase graph (done / in progress / blocked) with blocked reasons. |
| Configure engram | Enable cross-session memory: pick the artifact-store mode, set the cloud URL, and wire the engram MCP server into Copilot CLI and VS Code. |
| Configure code review | Wire Gentleman Guardian Angel (gga) into the project: write `.gga`, inject a curated `AGENTS.md` rules block tied to the active persona, and install the pre-commit review hook. Pick the provider; gga must be installed. |
| Install instructions | Write the curated scoped `*.instructions.md` into `~/.copilot/instructions/`. |
| Upgrade tools | Self-update capiko to the latest release, then restart. |
| Upgrade + sync | Upgrade, restart, and sync skills with the new catalog. |

## Headless CLI

Every install operation also runs non-interactively — no TUI, no TTY — so capiko fits
CI, dotfiles bootstrap, and automation. Add `--json` to any command for machine-readable
output.

| Command | What it does | Flags |
|---|---|---|
| `capiko-ai install` | Install every catalog skill + agent not already present (additive). | `--all`, `--json`, `--verbose` |
| `capiko-ai sync` | Overwrite all installed skills + agents to match the catalog; re-applies the persona, SDD, and engram blocks. | `--auto-repair`, `--json`, `--verbose` |
| `capiko-ai uninstall` | Remove every capiko-managed skill + agent and clear them from state (leaves persona/SDD/engram blocks alone). | `--all`, `--json`, `--verbose` |
| `capiko-ai doctor` | Read-only ecosystem health check; non-zero exit on failure. `--repair` re-applies the managed catalog when drift is found. | `--json`, `--repair`, `--verbose` |
| `capiko-ai sdd-status` · `sdd-continue` | Native SDD engine: print or advance the deterministic workflow state. | |
| `capiko-ai skill-registry` | Print the skill index for an orchestrator to resolve `SKILL.md` paths. | |
| `capiko-ai version` | Print the installed version and the targeted Copilot CLI version. | `-v`, `--version` |

**Exit codes** (`install` / `sync` / `uninstall`): `0` success · `1` error · `2` Copilot
CLI not found.

`--verbose` streams structured JSON-lines diagnostics (timestamp, event, result,
duration) to **stderr**, so stdout stays clean for scripting — available on every action
command above.

```bash
# Dotfiles / CI bootstrap
capiko-ai install --all --json
capiko-ai sync --auto-repair        # only writes when drift is detected
capiko-ai doctor --json || echo "capiko environment unhealthy"
```

Notes:

- `sync --auto-repair` checks for drift first and exits cleanly without writing when
  nothing changed — safe to call repeatedly from post-upgrade hooks.
- `uninstall` refuses (rather than guessing) when the state store is unavailable, so it
  never deletes files capiko didn't install.

## The SDD workflow

Once configured, ask Copilot to make a substantial change and it runs Spec-Driven
Development: `explore → propose → spec → design → tasks → apply → verify → archive`,
delegating each phase to a sub-agent with its assigned model. Artifacts live in the
OpenSpec store (`openspec/changes/<name>/`, `openspec/specs/`). Run `sdd-init` once
per project; `sdd-onboard` is a guided walkthrough.
