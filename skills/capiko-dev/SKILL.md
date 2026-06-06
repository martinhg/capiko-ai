---
name: capiko-dev
description: "capiko-ai codebase conventions and architecture. Trigger: any change to this repository."
---

## Workflow

- **Branch first.** Create a branch from `main` BEFORE the first edit. Never
  commit straight to `main`. Then commit, push, open a PR, wait for CI, squash-
  merge, delete the branch, and sync `main`.
- Conventional commits. **Never** add Co-Authored-By or any AI attribution.
- Quality gate before opening a PR: `gofmt -l .` clean, `go vet ./...` clean,
  `go test -race ./...` green.
- Tools: prefer `bat`/`rg`/`fd`/`eza` over `cat`/`grep`/`find`/`ls`.

## TUI golden tests

- Screen rendering is covered by golden files in `internal/tui/testdata/*.golden`.
- Regenerate with `go test ./internal/tui -update`, then **always inspect the diff**
  before committing — confirm only the intended lines changed.
- Tests force the ASCII color profile (see `view_test.go` `TestMain`) so goldens
  are deterministic across terminals and CI.

## Architecture patterns

- **Screens** implement `screen` (`Update(tea.Msg) (screen, tea.Cmd)` + `View()`).
  They return to the menu by emitting `backMsg`, and transition to another screen
  by **returning the next screen from `Update`** (e.g. detection → persona → SDD →
  selector). Test a screen by driving `Update` with `tea.Msg` directly.
- **Instruction blocks** (persona, SDD orchestrator) are marker-bound sections
  written into `~/.copilot/copilot-instructions.md` via `internal/instructions`.
  Render-when-changed, back up the file (`backup.CreateFiles`) before writing,
  then record in `internal/state`.
- **Skills** install verbatim into `~/.copilot/skills/<name>/SKILL.md`; **scoped
  instructions** into `~/.copilot/instructions/*.instructions.md`.
- **State** lives in `~/.capiko/state.json`; every mutation is snapshotted to
  `~/.capiko/backups/` first (snapshot-before-mutate).
- For testability, wrap exec / network / home-dir behind package-level function
  vars (seams), as `internal/release`, `internal/sysinfo`, `internal/copilot` do.
