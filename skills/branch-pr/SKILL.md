---
name: branch-pr
description: "Branch-first PR workflow for this repo. Trigger: starting a task or opening a PR."
---

## The flow

1. **Branch BEFORE editing.** `git checkout main && git pull --ff-only` then
   `git checkout -b <descriptive-name>`. Do this first, every task — committing to
   `main` by mistake is the most common slip here.
2. Implement in work-unit commits (see `work-unit-commits`).
3. Gate: `gofmt -l .` clean, `go vet ./...` clean, `go test -race ./...` green.
   Regenerate and inspect any affected goldens.
4. `git push -u origin <branch>` and open the PR with `gh pr create` (a clear
   What / Changes / Validation body).
5. Wait for CI green, then `gh pr merge <n> --squash --delete-branch`, and
   `git checkout main && git pull`.

## If you committed to main by mistake

Recover without losing work:
`git branch <name>` (snapshot HEAD) → `git reset --hard origin/main` →
`git checkout <name>` → push.
