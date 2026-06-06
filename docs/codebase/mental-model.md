# Mental Model

## What capiko is

capiko-ai is a **configurator** that mounts a company "capiko layer" onto the
**GitHub Copilot CLI** — the same pattern gentle-ai uses over Claude Code. It is a
terminal app (Go + Bubbletea) that **writes files** Copilot then auto-discovers.

| capiko IS | capiko is NOT |
|---|---|
| A file configurator: it writes skills, instruction blocks, and config. | An LLM. It never calls a model or generates content itself. |
| The owner of the install/sync/uninstall flow, backups, state, and self-update. | The thing that does the coding — **Copilot** does that. |
| The author of the SDD orchestrator + skills (guidance for Copilot). | The executor of SDD phases — Copilot's sub-agents run them. |

The rule of thumb: **capiko directs, Copilot executes.** Anything that needs an LLM
(generating a skill, analyzing a codebase, running an SDD phase) is shipped as
*guidance* (a skill or an instruction block) for Copilot, not implemented in Go.

## What it writes, and where

- **Skills** → `~/.copilot/skills/<name>/SKILL.md` (auto-discovered).
- **Persona / SDD orchestrator** → marker-bound blocks in
  `~/.copilot/copilot-instructions.md` (always-on global instructions).
- **Scoped instructions** → `~/.copilot/instructions/*.instructions.md` (applied
  per matching file via `applyTo`).
- **Its own state** → `~/.capiko/state.json`, with a snapshot to
  `~/.capiko/backups/` before every mutation.

## Why this matters

Knowing capiko has no LLM keeps features honest: when a feature seems to need
generation or code analysis, the answer is almost always "ship a skill that tells
Copilot to do it", not "add an API client to capiko".
