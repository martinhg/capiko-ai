---
name: chained-pr
description: "Split oversized changes into reviewable chained PRs. Trigger: a change is growing past ~400 lines or spans independent slices."
---

## When

A single PR is hard to review well past ~400 changed lines, or when a change has
naturally independent slices (e.g. engine → screen → wiring). Split it.

## How

- Slice by **dependency order**: each PR builds on the previous and is independently
  reviewable and mergeable. Example from this repo's SDD work:
  engine (#22) → config screen (#23) → phase skills (#24).
- Each slice keeps the suite green on its own (`go test -race ./...`).
- Land slices in order, merging each to `main` before the next (stacked-to-main),
  so review diffs stay small.
- State the plan in the first PR's body so reviewers know what's coming.

## Avoid

- A 1000-line PR "to save round-trips" — it gets a worse review.
- Slices that don't build without later ones (break the dependency order instead).
