# Repository Map

Use this when you know **what** you need to change but not **where** it belongs.

## Package ownership

| Path | Owns | Do not put here |
|---|---|---|
| `cmd/capiko-ai/` | Binary entry point; the `version`, `sdd-status`, `sdd-continue`, `skill-registry`, `doctor`, `install`, `sync`, and `uninstall` subcommands (resolved before the TUI); post-upgrade sync and re-exec after self-update. | Business logic, file mutation. |
| `internal/tui/` | Bubbletea model, screen routing, async messages, every interactive screen and flow, **plus the headless `InstallAll`/`UninstallAll` engines** the CLI commands call. | Domain rules, raw file IO details. |
| `internal/headless/` | Pure JSON/text renderers (`CommandResult`, `RenderJSON`/`RenderText`) for the headless `install`/`sync`/`uninstall` commands. | Domain logic, file IO. |
| `internal/skill/` | The skill domain: a `Skill`, and loading a catalog from any `fs.FS`. | UI, install targets. |
| `internal/catalog/` | The embedded skill **and** agent catalog (`go:embed`). | Go logic — only `SKILL.md` / `.agent.md` content. |
| `internal/copilot/` | Adapter to the Copilot CLI host (detect, list installed, install, uninstall). | UI, skills, instructions. |
| `internal/agent/` | The custom-agent domain: `.agent.md` SDD-phase agents installed to `~/.copilot/agents/`. | UI, file IO details. |
| `internal/state/` | `~/.capiko/state.json` persistence (skills, persona, SDD models, strict TDD, scoped flag, engram config). | UI, business flows. |
| `internal/backup/` | Snapshot/restore of skills and standalone files under `~/.capiko/backups/`. | UI. |
| `internal/instructions/` | Marker-bound block injection (`Inject`/`Render`/`Write`) into Copilot instruction files. | Feature-specific content. |
| `internal/persona/` | Persona content (embedded) + selection; renders via `instructions`. | File IO (delegated to `instructions`). |
| `internal/sdd/` | The SDD orchestrator block render + phase/model definitions. | UI, file IO. |
| `internal/sddstatus/` | The native SDD state engine: reads the OpenSpec store, computes status, renders JSON/markdown for `sdd-status`/`sdd-continue`. | UI, file mutation. |
| `internal/scoped/` | Curated `*.instructions.md` + install to `~/.copilot/instructions/`. | UI. |
| `internal/skillregistry/` | The skill-registry resolution engine behind `capiko-ai skill-registry` (scan user + project skills, index by trigger/path). | UI. |
| `internal/engram/` | engram detection, MCP wiring (Copilot CLI + VS Code), per-repo `.engram/config.json`, cloud config, and the server scaffold. | UI. |
| `internal/sysinfo/` | Environment detection (OS, tools, dependency versions, install hints, configs). | UI. |
| `internal/release/` | GitHub release version check + the self-update engine (brew/go/binary) + restart. | UI. |
| `internal/drift/` | Pure catalog-vs-`state.json` checksum comparison (which skills/agents/configs are stale). | UI, IO. |
| `internal/versions/` | Pinned external tool versions for Renovate (e.g. the Copilot CLI). | Logic. |

## Where common changes go

- A new **screen** → `internal/tui/<name>.go` (+ test + golden); wire it in `app.go`.
- A new **catalog skill** → `internal/catalog/skills/<name>/SKILL.md`.
- A new **managed instruction block** → render in its feature package, inject via
  `internal/instructions`, persist a flag in `internal/state`, back up via
  `internal/backup`, re-apply in `RunSync`.
- New **environment detection** → `internal/sysinfo`.
