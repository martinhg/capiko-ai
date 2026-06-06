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

You can also reach Persona, SDD config, and instructions from their own menu items
to change them later.

## The other menu items

| Item | What it does |
|---|---|
| Managed uninstall | List installed capiko skills; unmark to remove. |
| Sync configs | Overwrite all installed skills to match the catalog; re-applies the persona and SDD blocks. |
| Manage backups | Browse / restore / delete the snapshots taken before each change. |
| Configure SDD | Edit the per-phase model table and strict-TDD toggle. |
| Install instructions | Write the curated scoped `*.instructions.md` into `~/.copilot/instructions/`. |
| Upgrade tools | Self-update capiko to the latest release, then restart. |
| Upgrade + sync | Upgrade, restart, and sync skills with the new catalog. |

## The SDD workflow

Once configured, ask Copilot to make a substantial change and it runs Spec-Driven
Development: `explore → propose → spec → design → tasks → apply → verify → archive`,
delegating each phase to a sub-agent with its assigned model. Artifacts live in the
OpenSpec store (`openspec/changes/<name>/`, `openspec/specs/`). Run `sdd-init` once
per project; `sdd-onboard` is a guided walkthrough.
