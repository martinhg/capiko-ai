# The native SDD engine

capiko isn't only a Copilot configurator — it's a real SDD **engine**. The Spec-Driven
Development workflow ships as prompt contracts (the `sdd-shared` skill bundle), but the
state those contracts depend on is computed deterministically in Go, not reconstructed
by the model. That is the difference between a prompt that *describes* a process and a
tool that *runs* one.

## What it exposes

Two subcommands, resolved before the TUI launches (same pattern as `version`):

| Command | Returns |
|---|---|
| `capiko-ai sdd-status [change] --cwd <repo> --json` | The structured status object: change root, artifact paths, per-artifact state, task progress, dependency states, and the action context. |
| `capiko-ai sdd-continue [change]` | Dispatcher output: the next phase to run. |

The skills treat the native command as authoritative and fall back to reading the
OpenSpec files themselves only when the binary is unavailable.

## How `Resolve` works

`internal/sddstatus` reads the OpenSpec store and computes:

- **Artifact states** — each of proposal / spec / design / tasks / apply-progress /
  verify-report is `missing`, `done`, or `partial`.
- **Task progress** — parses the `tasks.md` checkboxes into done/total counts.
- **The dependency state machine** — `proposal → spec/design → tasks → apply → verify
  → archive`. `resolveNextRecommended` routes the planning phases deterministically
  (`propose | spec | design | tasks`) instead of emitting a generic "resolve-blockers"
  and making the coordinator infer the next step.
- **Apply state and verify detection** — whether apply is in progress and whether the
  verify report is "clearly passing".

The renderers emit `RenderJSON` (`schemaName: capiko.sdd-status`), `RenderMarkdown`,
and `RenderDispatcherMarkdown`.

## The OpenSpec path contract

Two distinct artifacts, not an inconsistency:

- `openspec/specs/` — the **canonical accumulated specs**, the source of truth, read
  as context.
- `openspec/changes/<change>/spec.md` — that change's **spec delta**, a single file.

The engine resolves a change's spec as `spec.md`. `apply-progress.md` and
`verify-report.md` live under the change directory and are written by the phase skills.
An **active change** is one present under `openspec/changes/` and not archived; when
there are zero active changes, the engine returns `sdd-new` and the triage gate — not
the engine — decides whether to act on it.

## Design notes

- The engine reads the **user's** project; capiko has no `openspec/` directory of its
  own, so tests run against fixtures.
- Subcommands resolve and exit before the full-screen TUI starts, so they're safe to
  pipe and script.
- `internal/sdd` renders the orchestrator block and model table; `internal/sddstatus`
  is the separate engine that computes state. Keep the two concerns apart.
