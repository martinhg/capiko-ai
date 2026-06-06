---
name: go-testing
description: "Go and Bubbletea testing patterns for this repo. Trigger: writing or reviewing tests, golden files, or coverage."
---

## Patterns

- **Table-driven** tests for multiple cases; name cases by scenario, use `t.Run`.
- **Filesystem**: always `t.TempDir()`; never touch the real home directory.
- **Seams over mocks**: swap package-level function vars (e.g. `lookPath`,
  `userHomeDir`, `runVersion`, `runInstall`, `reExec`) and restore via
  `t.Cleanup`. This is how exec/network/home paths are tested here.

## Bubbletea / TUI

- Test state transitions by calling `Model.Update(msg)` directly with a `tea.Msg`
  (use the `key(...)` helper for `tea.KeyMsg`). Assert the returned screen, state,
  and emitted `tea.Cmd` (call `cmd()` and type-assert the message).
- Test rendered output with **golden files** (`golden(t, name, view)`); update via
  `go test ./internal/tui -update` and inspect the diff.
- Keep screen tests **hermetic** — build the screen struct directly when you only
  need navigation, so you don't trigger real detection/exec.

## Run

- Run the narrow package first, then `go test -race ./...` before a PR.
