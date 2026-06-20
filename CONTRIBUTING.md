# Contributing to capiko-ai

Thanks for helping improve capiko-ai. This guide covers the workflow and the quality
gate so your change lands smoothly.

## Quick path

1. Fork and branch from `main` (e.g. `feat/<short-name>` or `fix/<short-name>`).
2. Make your change. Add or update tests — capiko is test-first.
3. Run the gate locally (all four must pass):

   ```bash
   gofmt -l .            # must print nothing
   go vet ./...
   go test -race ./...
   go build ./...
   ```

4. Commit with [Conventional Commits](https://www.conventionalcommits.org/)
   (`feat:`, `fix:`, `chore:`, `docs:`, …).
5. Open a PR against `main`. CI runs the same gate.

## The quality gate

| Check | Command | Why |
|-------|---------|-----|
| Formatting | `gofmt -l .` | Output must be empty — every file gofmt-clean. |
| Static analysis | `go vet ./...` | Catches suspicious constructs before review. |
| Tests | `go test -race ./...` | The full suite, with the race detector on. |
| Build | `go build ./...` | Every package compiles. |

## Conventions

- **Conventional Commits.** The commit type drives changelog and release tooling.
- **No AI attribution** in commit messages (no `Co-Authored-By` trailers).
- **PRs only — never push directly to `main`.** Even release bumps go through a PR.
- **Keep PRs reviewable.** Large changes are split into chained PRs (the SDD
  review-workload guard forecasts this automatically).
- **Tests travel with code.** New behavior ships with tests in the same change.

## Where things live

- `cmd/capiko-ai/` — binary entry point and subcommands.
- `internal/` — all packages (see the [Project layout](README.md#-project-layout)).
- `internal/catalog/` — the skills and agents capiko ships to users (embedded in the binary).
- `internal/tui/testdata/` — golden snapshots for TUI rendering; refresh with
  `go test ./internal/tui -update` when a rendering change is intended.

Read [`AGENTS.md`](AGENTS.md) and [`docs/codebase/`](docs/codebase/) for the mental model
and architecture before changing code.

## Reporting bugs and requesting features

Open a [GitHub issue](https://github.com/martinhg/capiko-ai/issues) with what you
expected, what happened, and the output of `capiko-ai doctor`. For security issues, see
[SECURITY.md](SECURITY.md) — do not open a public issue.
