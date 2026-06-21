# Architecture

The patterns that recur across capiko. Match them when adding features.

## TUI screens

- Every screen implements `screen` (`Update(tea.Msg) (screen, tea.Cmd)` + `View()`).
- A screen returns to the menu by emitting `backMsg`, and **transitions to another
  screen by returning the next screen from `Update`** (e.g. detection → persona →
  SDD → selector). There is no central navigation stack.
- Long work (install, sync, upgrade) runs in a `tea.Cmd` and reports back via a
  result message the screen handles; the screen shows confirm → applying → done.
- The main menu lives in one fixed-width double-border box (`internal/tui/styles.go`).

## Managed instruction blocks

Persona and the SDD orchestrator are **marker-bound blocks** in
`~/.copilot/copilot-instructions.md` (always loaded by Copilot):

1. The feature package renders its block (`persona.Render`, `sdd.Render`).
2. `internal/instructions.Render` injects it between `<!-- capiko:x:start/end -->`,
   reporting whether it changed; content outside the markers is preserved.
3. If changed: snapshot the file (`backup.CreateFiles`), then `instructions.Write`
   atomically.
4. Record the choice in `internal/state`.
5. `RunSync` re-applies all managed blocks (the InjectForSync equivalent).

## State and backups

- `internal/state` owns `~/.capiko/state.json` (atomic writes; per-skill checksums).
- Every mutation is **snapshot-before-mutate**: `internal/backup` copies the
  affected skills, agent files, and standalone files to `~/.capiko/backups/<id>/`
  (with a manifest) first, so any change is restorable from **Manage backups**.
  `CreateWithAgents` keeps skills and agents in one backup id, so install/uninstall
  restore symmetrically.

## Versioning and self-update

- `Version` (in `internal/tui`) is set by goreleaser ldflags; falls back to
  `debug.ReadBuildInfo()` then a base version, so it never shows a pseudo-version.
- `internal/release` checks GitHub releases and self-updates (brew/go/binary),
  then re-execs. (Note for the SDD model table: Copilot's Task tool caps sub-agents
  to the session model, so the per-phase assignments are guidance the orchestrator
  honors, not a hard runtime switch.)

## Testing

- Table-driven tests; `t.TempDir()` for the filesystem; never the real home.
- Swap exec/network/home behind **package-level function vars (seams)**, restored
  with `t.Cleanup`.
- TUI output is covered by **golden files** under `internal/tui/testdata/`
  (ASCII profile, deterministic). Regenerate with `go test ./internal/tui -update`
  and inspect the diff.
