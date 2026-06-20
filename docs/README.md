# capiko-ai docs

The project's knowledge base, in three layers.

## For users

- [usage.md](usage.md) — the install flow, the menu, the headless CLI, and the SDD workflow.
- [capabilities.md](capabilities.md) — the full feature tour: everything capiko can do.
- [engram-cloud-setup.md](engram-cloud-setup.md) — standing up shared team memory.
- The root [README](../README.md) — install, quickstart, headless CLI, status.

## For maintainers

- [release.md](release.md) — cutting a release, rollback.
- [native-sdd-engine.md](native-sdd-engine.md) — how the native SDD state engine works.

## For contributors (codebase guide)

Start with the root [CONTRIBUTING.md](../CONTRIBUTING.md) for the workflow and the gate,
then read these before changing the code:

- [codebase/mental-model.md](codebase/mental-model.md) — what capiko is and is not.
- [codebase/repository-map.md](codebase/repository-map.md) — which package owns
  what, and where common changes go.
- [codebase/architecture.md](codebase/architecture.md) — the recurring patterns
  (screens, instruction blocks, state/backups, testing).
