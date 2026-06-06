# Epic: Native SDD Engine

Make `capiko-ai` a real SDD **engine** — not only a Copilot configurator. Today
capiko ships the SDD workflow as prompt contracts (the `sdd-shared` bundle), and
the agent reconstructs state by reading the OpenSpec files itself. gentle-ai
instead computes that state deterministically in Go (`gentle-ai sdd-status`) and
the skills treat it as authoritative. This epic gives capiko the same engine.

## Goal

A Go package that computes SDD state from `openspec/`, exposed as:

- `capiko-ai sdd-status [change] --cwd <repo> --json` — the structured status
  object (the schema already defined in `sdd-shared/sdd-status-contract.md`).
- `capiko-ai sdd-continue [change]` — dispatcher routing output (next phase).

Then flip `sdd-shared/sdd-status-contract.md` from "No Native Engine" to "prefer
the native command; fall back to the prompt contract when unavailable".

## Reference

Mirrors gentle-ai's `internal/sddstatus/status.go` (`Resolve(opts) (Status,
error)`, `ListActiveOpenSpecChanges`, renderers). gentle-ai's `internal/planner`
is **not** relevant — that is its component-install dependency resolver
(graph + topological sort over installable components), a different concern;
capiko's skills are flat and independent, so there is nothing to topologically
resolve.

## Slices (each a PR, dependency-ordered)

1. **Status model + OpenSpec reader** — new `internal/sddstatus` package: the
   `Status` types and the path/change discovery (`ListActiveOpenSpecChanges`,
   `resolveArtifactPaths`). Pure, no resolution logic yet.
2. **`Resolve(opts)` — the brain** — artifact states (missing/done/partial),
   `tasks.md` checkbox parsing → task progress, the dependency state machine
   (proposal → specs → tasks → apply → verify → archive), apply state,
   `nextRecommended`, `blockedReasons`, and verify-report "clearly passing"
   detection.
3. **Renderers** — `RenderJSON` (`schemaName: capiko.sdd-status`),
   `RenderMarkdown`, `RenderDispatcherMarkdown`.
4. **CLI subcommands** — `case "sdd-status"` / `case "sdd-continue"` in
   `cmd/capiko-ai/main.go`, resolved before the TUI launches (same pattern as
   `version`), with `--cwd` and `--json` flags.
5. **Flip the contract** — update `sdd-shared/sdd-status-contract.md` and the
   phase-skill `## Gate` blocks to prefer the native command.

## Key decisions & risks

- **OpenSpec path contract.** Two distinct artifacts, not an inconsistency:
  `openspec/specs/` (top-level directory) holds the **canonical accumulated specs**
  (the source of truth, read as context), while `openspec/changes/<change>/spec.md`
  is that change's **spec delta** (a single file capiko writes). The engine
  resolves the change's spec as `spec.md`; a `specs/` directory under a change is
  gentle-ai's per-capability layout, not capiko's, and is ignored. `apply-progress.md`
  and `verify-report.md` live under the change dir; the phase skills should write
  them.
- **`schemaName: capiko.sdd-status`** — capiko's own, not gentle-ai's.
- **Active change** = present under `openspec/changes/` and not archived.
- **TUI safety** — subcommands resolve and exit before the full-screen TUI starts.
- capiko has no `openspec/` dir of its own (it does not dogfood SDD); the engine
  reads the user's project, and tests use fixtures.
- `internal/sdd` today only renders model assignments — the engine is greenfield
  in a new `internal/sddstatus` package.

## Definition of done

`capiko-ai sdd-status --cwd <repo> --json` emits the schema that `sdd-shared`
consumes, the skills prefer it over inference, and capiko stops being only a
configurator: it becomes an SDD engine.
