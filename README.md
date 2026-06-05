# capiko-ai

A terminal configurator that supercharges the **GitHub Copilot CLI** with a
company layer of skills, workflows, and conventions — the same pattern
[gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) uses over Claude
Code, focused on Copilot.

capiko-ai does not replace Copilot. It writes a curated set of `SKILL.md` files
into Copilot's skills directory, which Copilot then auto-discovers. The actual
coding still happens inside Copilot.

## Quick path

```bash
# 1. Requirements: Go 1.26+, and the GitHub Copilot CLI, authenticated.
npm install -g @github/copilot
copilot            # run once, then /login, then /exit

# 2. Build and run
go build -o capiko-ai .
./capiko-ai

# 3. In the menu: Start installation → pick skills → enter.
#    Then verify from a neutral directory:
cd /tmp && copilot -p "list capiko skills" --allow-all-tools --add-dir ~/.copilot
```

## Menu

| Option | What it does |
|--------|--------------|
| Start installation | Pick skills to install or remove (declarative checkbox reconcile). |
| Managed uninstall | List installed capiko skills and remove the ones you unmark. |
| Sync configs | Overwrite all installed skills to match the current catalog. |
| Manage backups | Browse, restore, or delete snapshots taken before each change. |
| Upgrade tools | _Coming soon._ |
| Upgrade + sync | _Coming soon._ |

Navigation: `↑/↓` move · `space` toggle · `enter` apply · `q` back/quit.

## How it works

| Concern | Detail |
|---------|--------|
| Catalog | Skills ship embedded in the binary (`go:embed`). Edit `internal/catalog/skills/*/SKILL.md` and rebuild to change what is offered. |
| Install target | `~/.copilot/skills/<name>/SKILL.md`. Copilot auto-discovers skills there — no registration needed. |
| State | `~/.capiko/state.json` records what capiko installed, with a content checksum per skill. |
| Backups | Every change snapshots the affected skills to `~/.capiko/backups/<id>/` first. If a backup fails, the change aborts. |

## Project layout

| Package | Responsibility |
|---------|----------------|
| `internal/skill` | Domain: a skill, and loading a catalog from any `fs.FS`. |
| `internal/catalog` | The embedded skill catalog (`go:embed`). |
| `internal/copilot` | Adapter to the Copilot CLI host (detect, list, uninstall). |
| `internal/state` | Persistent state in `~/.capiko/state.json` (atomic writes). |
| `internal/backup` | Snapshot / restore of skills under `~/.capiko/backups`. |
| `internal/tui` | Bubbletea screens and the menu router. |

## Development

```bash
go test ./...                         # run the suite
go test -cover ./...                  # with coverage
go test ./internal/tui -update        # refresh View() golden snapshots
go vet ./...                          # static checks
```

UI rendering is covered by golden snapshots in `internal/tui/testdata`. They use
a plain color profile so they stay deterministic; colors only render in a real
terminal.

## Status

Working: configurator (install / uninstall / sync), backups with restore,
persistent state, embedded catalog, menu UI. Planned: upgrade detection via
checksums, a real version-update check, and a release pipeline.
