<div align="center">

<pre>
                    _ _                  _
   ___ __ _ _ __ (_) | _____         __ _(_)
  / __/ _` | '_ \| | |/ / _ \ _____ / _` | |
 | (_| (_| | |_) | |   < (_) |_____| (_| | |
  \___\__,_| .__/|_|_|\_\___/       \__,_|_|
           |_|        (•ᴥ•)
</pre>

# capiko-ai

**Turn the GitHub Copilot CLI into a senior teammate that already knows your team's playbook.**

[![CI](https://github.com/martinhg/capiko-ai/actions/workflows/ci.yml/badge.svg)](https://github.com/martinhg/capiko-ai/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/martinhg/capiko-ai?sort=semver)](https://github.com/martinhg/capiko-ai/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/martinhg/capiko-ai)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Platforms](https://img.shields.io/badge/platforms-macOS%20%C2%B7%20Linux%20%C2%B7%20Windows-informational)

[Install](#-install) ·
[Quickstart](#-quickstart) ·
[Headless CLI](#-headless-cli) ·
[SDD workflow](#-spec-driven-development) ·
[How it works](#-how-it-works) ·
[Docs](#-documentation) ·
[Contributing](#-contributing) ·
[llms.txt](llms.txt)

</div>

---

capiko-ai mounts a whole company layer onto Copilot in one pass: a curated catalog of
skills, a teaching-first persona, cross-session memory, and a full Spec-Driven
Development workflow with deterministic state and per-phase model routing. Run one TUI
(or one headless command), pick what you want, and Copilot comes back onboarded —
conventions, memory, and a spec-driven process included.

It never replaces Copilot and **it never calls a model itself**. capiko **directs**;
Copilot **executes**. Everything that needs an LLM ships as guidance Copilot
auto-discovers — so what capiko installs is reproducible, auditable, and yours.

<div align="center">

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

</div>

## ✨ What you get

| Layer | What capiko installs | Where it lands |
|---|---|---|
| **Skills** | A curated catalog of `SKILL.md` files Copilot auto-discovers — SDD phases, a skill creator, a codebase-docs generator, and more. | `~/.copilot/skills/<name>/` |
| **Persona** | A teaching-first (Rioplatense), Neutral, or None instruction block that shapes how Copilot talks and reasons. | `~/.copilot/copilot-instructions.md` |
| **SDD workflow** | An 8-phase Spec-Driven Development pipeline, each phase delegated to a sub-agent on its own assigned model, with strict-TDD forwarding and a review-workload guard. | instructions block + custom agents |
| **Copilot agents** | `.agent.md` SDD-phase agents — the real delegation targets the workflow routes to. | `~/.copilot/agents/` |
| **Scoped instructions** | `*.instructions.md` with `applyTo` globs, applied per matching file. | `~/.copilot/instructions/` |
| **Engram memory** | Wires the [engram](https://github.com/Gentleman-Programming/engram) MCP server into Copilot CLI **and** VS Code, with local + cloud (`hybrid`) cross-session memory. | `~/.copilot/mcp-config.json`; VS Code `mcp.json` |
| **Safety net** | Snapshot-before-mutate backups, persistent state with per-skill checksums, drift detection, and self-update. | `~/.capiko/` |

And capiko ships **its own native SDD engine** in Go — `capiko-ai sdd-status` /
`sdd-continue` compute the workflow state deterministically instead of asking the model
to reconstruct it. That is the difference between a prompt that *describes* a process
and a tool that *runs* one.

## 📦 Install

<table>
<tr><th>Platform</th><th>Command</th></tr>
<tr><td>macOS / Linux (Homebrew)</td><td>

```bash
brew install martinhg/homebrew-tap/capiko-ai
```

</td></tr>
<tr><td>macOS / Linux (script)</td><td>

```bash
curl -sL https://raw.githubusercontent.com/martinhg/capiko-ai/main/scripts/install.sh | bash
```

</td></tr>
<tr><td>Windows (PowerShell)</td><td>

```powershell
irm https://raw.githubusercontent.com/martinhg/capiko-ai/main/scripts/install.ps1 | iex
```

</td></tr>
<tr><td>Windows (Scoop)</td><td>

```powershell
scoop bucket add capiko-ai https://github.com/martinhg/scoop-bucket
scoop install capiko-ai
```

</td></tr>
<tr><td>From source (Go 1.26+)</td><td>

```bash
go install github.com/martinhg/capiko-ai/cmd/capiko-ai@latest
```

</td></tr>
</table>

Pre-built binaries for every platform are attached to each
[release](https://github.com/martinhg/capiko-ai/releases). Run `capiko-ai version` to
print the installed version and the Copilot CLI version it targets.

## 🚀 Quickstart

**Requirements:** the [GitHub Copilot CLI](https://github.com/github/copilot), authenticated.

```bash
npm install -g @github/copilot
copilot            # run once, then /login, then /exit
```

### Interactive (TUI)

```bash
capiko-ai
# In the menu: Start installation → System Detection → Persona →
# Configure SDD → pick skills → Review and Confirm → apply.
```

### Headless (scripts, CI, dotfiles)

```bash
capiko-ai install --all     # install the whole catalog, no prompts
capiko-ai sync              # reconcile installed configs to the catalog
```

Either way, verify from a neutral directory:

```bash
cd /tmp && copilot -p "list capiko skills" --allow-all-tools --add-dir ~/.copilot
```

That's it — Copilot now has your skills, persona, and SDD workflow live. Ask it to make
a substantial change and watch it run the full spec-driven pipeline.

## 🤖 Headless CLI

Every install operation is scriptable — **no TUI, no TTY required** — so capiko drops
straight into CI, dotfiles bootstrap, and automation. All commands accept `--json` for
machine-readable output.

| Command | What it does | Flags |
|---|---|---|
| `capiko-ai install` | Install every catalog skill + agent not already present (additive — never touches what's already there). | `--all`, `--json` |
| `capiko-ai sync` | Overwrite all installed skills + agents to match the catalog; re-applies the persona, SDD, and engram blocks. Warns when a capiko-managed engram binary is behind the recommended version. | `--auto-repair`, `--json` |
| `capiko-ai uninstall` | Remove every capiko-managed skill + agent and clear them from state. Leaves persona/SDD/engram blocks untouched. | `--all`, `--json` |
| `capiko-ai doctor` | Read-only ecosystem health check (OS, prereqs, Copilot init, state, drift, engram). Non-zero exit on failure. | `--json` |
| `capiko-ai sdd-status` · `sdd-continue` | Native SDD engine: print or advance the deterministic workflow state. | |
| `capiko-ai skill-registry` | Print the skill index so an orchestrator can resolve exact `SKILL.md` paths. | |
| `capiko-ai version` | Print the installed version and the targeted Copilot CLI version. | `-v`, `--version` |

**Exit codes** (`install` / `sync` / `uninstall`): `0` success · `1` error · `2` Copilot CLI not found.

```bash
# Dotfiles / CI bootstrap
capiko-ai install --all --json
capiko-ai sync --auto-repair        # only writes when drift is detected
capiko-ai doctor --json || echo "capiko environment unhealthy"
```

> `--auto-repair` checks for drift first and exits cleanly without writing when nothing
> changed — safe to call repeatedly from post-upgrade hooks. When the state store is
> unavailable, `uninstall` refuses rather than risk deleting unmanaged files.

## 🧭 The menu

| Option | What it does |
|--------|--------------|
| **Start installation** | The guided flow: System Detection → Persona → SDD config → pick skills → Review and Confirm → apply. Detection reports your OS/shell, tools, and dependency versions (with one-click install of the safe ones); Persona picks Capiko / Neutral / None. |
| **Managed uninstall** | List installed capiko skills; unmark to remove. |
| **Sync configs** | Overwrite all installed skills to match the catalog; re-applies the persona, SDD, and engram blocks. |
| **Manage backups** | Browse, restore, or delete the snapshots taken before each change. |
| **Configure SDD** | Assign a model per SDD phase, set per-phase reasoning effort, and toggle strict TDD. |
| **Configure engram** | Enable cross-session memory: pick the artifact-store mode, set the cloud URL, wire the MCP server into Copilot CLI and VS Code. |
| **Upgrade tools** / **Upgrade + sync** | Self-update capiko, restart, and optionally re-sync against the new catalog. |
| **Install instructions** | Write the curated scoped `*.instructions.md` files. |

Navigation: `↑/↓` move · `enter` select · `q` quit. (Inside the skill selector, `space`
toggles and `enter` applies.)

## 🧪 Spec-Driven Development

This is capiko's headline feature. Once configured, ask Copilot to make a substantial
change and it runs an eight-phase pipeline instead of free-styling:

```text
explore → propose → spec → design → tasks → apply → verify → archive
```

- **Every phase is delegated** to a sub-agent loaded with that phase's skill, on a model
  assigned per phase — architecture phases on a top model, mechanical phases
  auto-downgraded to save tokens, with per-phase reasoning effort.
- **State is computed, not guessed.** The native Go engine (`capiko-ai sdd-status`)
  returns a structured status object; the skills prefer it over re-reading markdown.
- **Strict TDD travels with the work.** When the toggle is on, the failing-test-first
  protocol is forwarded into the apply/verify handoff — not just mentioned once.
- **Oversized changes get split.** A review-workload guard forecasts the diff before
  apply and steers large changes into reviewable chained PRs.

Run `sdd-init` once per project; `sdd-onboard` is a guided walkthrough that teaches the
whole cycle on your real code. See [docs/usage.md](docs/usage.md) for the full flow.

## ⚙️ How it works

| Concern | Detail |
|---------|--------|
| **No LLM inside** | capiko writes files; Copilot does the thinking. Anything that needs a model ships as a skill or an instruction block — never as a Go API client. |
| **Catalog** | Skills and agents ship embedded in the binary (`go:embed`). Edit `internal/catalog/` and rebuild to change what's offered. |
| **Install target** | `~/.copilot/skills/`, `~/.copilot/agents/`, `~/.copilot/instructions/`, and marker-bound blocks in `~/.copilot/copilot-instructions.md`. Copilot auto-discovers all of it. |
| **State** | `~/.capiko/state.json` records what capiko installed, with a content checksum per skill — so it knows what's yours, what drifted, and what to re-apply. |
| **Backups** | Every change snapshots the affected files to `~/.capiko/backups/<id>/` first. If a backup fails, the change aborts. Restore any of them from **Manage backups**. |
| **Self-update** | `internal/release` checks GitHub releases and updates in place (brew/go/binary), then re-execs. |

## 🗂️ Project layout

| Package | Responsibility |
|---------|----------------|
| `cmd/capiko-ai/` | Binary entry point and every subcommand: `install`, `sync`, `uninstall`, `doctor`, `sdd-status`, `sdd-continue`, `skill-registry`, `version`. |
| `internal/tui/` | Bubbletea screens, the menu router, every interactive flow, and the headless install/sync/uninstall engines. |
| `internal/headless` | Pure JSON/text renderers for the headless commands. |
| `internal/skill` · `internal/catalog` | The skill domain, and the embedded catalog of skills and agents. |
| `internal/copilot` | Adapter to the Copilot CLI host (detect, list, install, uninstall). |
| `internal/persona` · `internal/instructions` | Persona content and marker-bound instruction-block injection. |
| `internal/sdd` · `internal/sddstatus` | The SDD orchestrator block + model table, and the native state engine. |
| `internal/scoped` · `internal/skillregistry` | Scoped `*.instructions.md`, and the skill-registry resolution engine. |
| `internal/engram` | engram detection, MCP wiring (Copilot + VS Code), per-repo config, and cloud setup. |
| `internal/state` · `internal/backup` · `internal/drift` | Persistent state, snapshot/restore, and catalog-vs-state drift. |
| `internal/sysinfo` · `internal/release` · `internal/versions` | Environment detection, self-update, and pinned tool versions. |

## 🛠️ Development

```bash
go test -race ./...                   # run the suite (race detector on)
go test -cover ./...                  # with coverage
go test ./internal/tui -update        # refresh View() golden snapshots
go vet ./...                          # static checks
gofmt -l .                            # formatting check
```

UI rendering is covered by golden snapshots in `internal/tui/testdata`. They use a plain
color profile so they stay deterministic; colors only render in a real terminal. Read
[`AGENTS.md`](AGENTS.md) and [`docs/codebase/`](docs/codebase/) before changing the code,
and [`CONTRIBUTING.md`](CONTRIBUTING.md) for the contribution workflow.

## 📚 Documentation

| Guide | What's inside |
|-------|---------------|
| **[docs/usage.md](docs/usage.md)** | The install flow, the menu, the headless CLI, and the SDD workflow. |
| **[docs/capabilities.md](docs/capabilities.md)** | The full feature tour: everything capiko can do. |
| **[docs/native-sdd-engine.md](docs/native-sdd-engine.md)** | How the native SDD engine works. |
| **[docs/engram-cloud-setup.md](docs/engram-cloud-setup.md)** | Standing up shared team memory. |
| **[docs/release.md](docs/release.md)** | Cutting a release. |
| **[docs/codebase/](docs/codebase/)** | The contributor's guide to the codebase. |

## 🤝 Contributing

Issues and PRs are welcome. The gate is simple — `gofmt`, `go vet`, `go test -race`, and
`go build` must all pass — and commits follow [Conventional Commits](https://www.conventionalcommits.org/).
See **[CONTRIBUTING.md](CONTRIBUTING.md)** for the full workflow and **[SECURITY.md](SECURITY.md)**
to report a vulnerability.

## 📄 License

[MIT](LICENSE) © capiko-ai contributors.

<div align="center"><sub>capiko directs · Copilot executes · the human always leads</sub></div>
