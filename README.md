# capiko-ai

[![CI](https://github.com/martinhg/capiko-ai/actions/workflows/ci.yml/badge.svg)](https://github.com/martinhg/capiko-ai/actions/workflows/ci.yml)

**Turn the GitHub Copilot CLI into a senior teammate that already knows your team's
playbook.**

capiko-ai is a terminal app that mounts a whole company layer onto Copilot in one
pass: a curated catalog of skills, a teaching-first persona, cross-session memory,
and a full Spec-Driven Development workflow with deterministic state and per-phase
model routing. You run one TUI, pick what you want, and Copilot comes back
onboarded — conventions, memory, and a spec-driven process included.

It never replaces Copilot and it never calls a model itself. capiko **directs**;
Copilot **executes**. Everything that needs an LLM ships as guidance Copilot
auto-discovers — so what capiko installs is reproducible, auditable, and yours.

```text
        ╭──────────────────────────────────────────────╮
        │   (•ᴥ•)   capiko-ai                           │
        │                                              │
        │   ▸ Start installation                       │
        │     Managed uninstall                        │
        │     Sync configs                             │
        │     Configure SDD                            │
        │     Configure engram                         │
        │     Manage backups                           │
        ╰──────────────────────────────────────────────╯
```

## What you get

| Layer | What capiko installs | Where it lands |
|---|---|---|
| **Skills** | A curated catalog of `SKILL.md` files Copilot auto-discovers — SDD phases, a skill creator, a codebase-docs generator, and more. | `~/.copilot/skills/<name>/` |
| **Persona** | A teaching-first (Rioplatense), Neutral, or None instruction block that shapes how Copilot talks and reasons. | `~/.copilot/copilot-instructions.md` |
| **SDD workflow** | An 8-phase Spec-Driven Development pipeline, each phase delegated to a sub-agent on its own assigned model, with strict-TDD forwarding and a review-workload guard. | instructions block + custom agents |
| **Copilot agents** | `.agent.md` SDD-phase agents — the real delegation targets the workflow routes to. | `~/.copilot/agents/` |
| **Scoped instructions** | `*.instructions.md` with `applyTo` globs, applied per matching file. | `~/.copilot/instructions/` |
| **Engram memory** | Wires the [engram](https://github.com/Gentleman-Programming/engram) MCP server into Copilot CLI **and** VS Code, with local + cloud (`hybrid`) cross-session memory. | `~/.copilot/mcp-config.json`; VS Code `.vscode/mcp.json` (workspace) or `Code/User/mcp.json` (user) |
| **Safety net** | Snapshot-before-mutate backups, persistent state with per-skill checksums, drift detection, and self-update. | `~/.capiko/` |

And capiko ships **its own native SDD engine** in Go — `capiko-ai sdd-status` /
`sdd-continue` compute the workflow state deterministically instead of asking the
model to reconstruct it. That is the difference between a prompt that *describes* a
process and a tool that *runs* one.

## Install

**Homebrew (macOS / Linux)**

```bash
brew install martinhg/homebrew-tap/capiko-ai
```

**Install script (macOS / Linux)**

```bash
curl -sL https://raw.githubusercontent.com/martinhg/capiko-ai/main/scripts/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/martinhg/capiko-ai/main/scripts/install.ps1 | iex
# or, with Scoop:
scoop bucket add capiko-ai https://github.com/martinhg/scoop-bucket
scoop install capiko-ai
```

**From source**

```bash
go install github.com/martinhg/capiko-ai/cmd/capiko-ai@latest
```

Pre-built binaries for every platform are attached to each
[release](https://github.com/martinhg/capiko-ai/releases). Run `capiko-ai version`
to print the installed version.

## Quick path

```bash
# 1. Requirements: Go 1.26+, and the GitHub Copilot CLI, authenticated.
npm install -g @github/copilot
copilot            # run once, then /login, then /exit

# 2. Build and run
go build -o capiko-ai ./cmd/capiko-ai
./capiko-ai

# 3. In the menu: Start installation → System Detection → Persona →
#    Configure SDD → pick skills → Review and Confirm → apply.

# 4. Verify from a neutral directory:
cd /tmp && copilot -p "list capiko skills" --allow-all-tools --add-dir ~/.copilot
```

That's it — Copilot now has your skills, persona, and SDD workflow live. Ask it to
make a substantial change and watch it run the full spec-driven pipeline.

## The menu

| Option | What it does |
|--------|--------------|
| **Start installation** | The guided flow: System Detection → Persona → SDD config → pick skills → Review and Confirm → apply. Detection reports your OS/shell, tools, and dependency versions (with one-click install of the safe ones); Persona picks Capiko / Neutral / None. |
| **Managed uninstall** | List installed capiko skills; unmark to remove. |
| **Sync configs** | Overwrite all installed skills to match the catalog; re-applies the persona, SDD, and engram blocks. |
| **Manage backups** | Browse, restore, or delete the snapshots taken before each change. |
| **Configure SDD** | Assign a model per SDD phase and toggle strict TDD. |
| **Configure engram** | Enable cross-session memory: pick the artifact-store mode, set the cloud URL, wire the MCP server into Copilot CLI and VS Code. |
| **Upgrade tools** / **Upgrade + sync** | Self-update capiko, restart, and optionally re-sync against the new catalog. |
| **Install instructions** | Write the curated scoped `*.instructions.md` files. |

Navigation: `↑/↓` move · `enter` select · `q` quit. (Inside the skill selector,
`space` toggles and `enter` applies.)

## The Spec-Driven Development workflow

This is capiko's headline feature. Once configured, ask Copilot to make a
substantial change and it runs an eight-phase pipeline instead of free-styling:

```text
explore → propose → spec → design → tasks → apply → verify → archive
```

- **Every phase is delegated** to a sub-agent loaded with that phase's skill, on a
  model assigned per phase — architecture phases on a top model, mechanical phases
  auto-downgraded to save tokens.
- **State is computed, not guessed.** The native Go engine (`capiko-ai sdd-status`)
  reads the OpenSpec store and returns a structured status object; the skills prefer
  it over re-reading markdown.
- **Strict TDD travels with the work.** When the toggle is on, the failing-test-first
  protocol is forwarded into the apply/verify handoff — not just mentioned once.
- **Oversized changes get split.** A review-workload guard forecasts the diff before
  apply and steers large changes into reviewable chained PRs.

Run `sdd-init` once per project; `sdd-onboard` is a guided walkthrough that teaches
the whole cycle on your real code. See [docs/usage.md](docs/usage.md) for the full flow.

## How it works

| Concern | Detail |
|---------|--------|
| **No LLM inside** | capiko writes files; Copilot does the thinking. Anything that needs a model ships as a skill or an instruction block — never as a Go API client. |
| **Catalog** | Skills and agents ship embedded in the binary (`go:embed`). Edit `internal/catalog/` and rebuild to change what's offered. |
| **Install target** | `~/.copilot/skills/`, `~/.copilot/agents/`, `~/.copilot/instructions/`, and marker-bound blocks in `~/.copilot/copilot-instructions.md`. Copilot auto-discovers all of it. |
| **State** | `~/.capiko/state.json` records what capiko installed, with a content checksum per skill — so it knows what's yours, what drifted, and what to re-apply. |
| **Backups** | Every change snapshots the affected files to `~/.capiko/backups/<id>/` first. If a backup fails, the change aborts. Restore any of them from **Manage backups**. |
| **Self-update** | `internal/release` checks GitHub releases and updates in place (brew/go/binary), then re-execs. |

## Project layout

| Package | Responsibility |
|---------|----------------|
| `cmd/capiko-ai/` | Binary entry point; the `version`, `sdd-status`, `sdd-continue`, and `skill-registry` subcommands. |
| `internal/tui/` | Bubbletea screens, the menu router, and every interactive flow. |
| `internal/skill` · `internal/catalog` | The skill domain, and the embedded catalog of skills and agents. |
| `internal/copilot` | Adapter to the Copilot CLI host (detect, list, uninstall). |
| `internal/persona` · `internal/instructions` | Persona content and marker-bound instruction-block injection. |
| `internal/sdd` · `internal/sddstatus` | The SDD orchestrator block + model table, and the native state engine. |
| `internal/scoped` · `internal/skillregistry` | Scoped `*.instructions.md`, and the skill-registry resolution engine. |
| `internal/engram` | engram detection, MCP wiring (Copilot + VS Code), per-repo config, and cloud setup. |
| `internal/state` · `internal/backup` · `internal/drift` | Persistent state, snapshot/restore, and catalog-vs-state drift. |
| `internal/sysinfo` · `internal/release` · `internal/versions` | Environment detection, self-update, and pinned tool versions. |

## Development

```bash
go test ./...                         # run the suite
go test -cover ./...                  # with coverage
go test ./internal/tui -update        # refresh View() golden snapshots
go vet ./...                          # static checks
gofmt -l .                            # formatting check
```

UI rendering is covered by golden snapshots in `internal/tui/testdata`. They use a
plain color profile so they stay deterministic; colors only render in a real
terminal. Read [`AGENTS.md`](AGENTS.md) and [`docs/codebase/`](docs/codebase/) before
changing the code.

## Documentation

- **[docs/usage.md](docs/usage.md)** — the install flow, the menu, and the SDD workflow.
- **[docs/capabilities.md](docs/capabilities.md)** — the full feature tour: everything capiko can do.
- **[docs/native-sdd-engine.md](docs/native-sdd-engine.md)** — how the native SDD engine works.
- **[docs/engram-cloud-setup.md](docs/engram-cloud-setup.md)** — standing up shared team memory.
- **[docs/release.md](docs/release.md)** — cutting a release.
- **[docs/codebase/](docs/codebase/)** — the contributor's guide to the codebase.

## Status

**Working:** the configurator (install / uninstall / sync), the persona system, the
full SDD workflow with a native Go engine and per-phase model routing, Copilot custom
agents, scoped instructions, engram memory (local + cloud, `hybrid` default), system
detection with one-click dependency install, snapshot/restore backups, drift
detection, self-update, and a multi-platform release pipeline (Homebrew, Scoop,
install scripts).

**Planned:** Copilot CLI model selection surfaced in the SDD config when the CLI
exposes one.
