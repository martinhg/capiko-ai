---
name: work-unit-commits
description: "Plan commits as small, reviewable work units. Trigger: implementing a change or splitting a PR."
---

## Principle

Each commit is ONE coherent, reviewable unit — code and its tests together, in a
state that builds and passes. A reviewer should understand it from the message
and diff alone.

## How

- Slice a PR into logical commits by concern, not by file. Example from this repo:
  `feat: add sysinfo detection` · `feat: System Detection screen` rather than one
  dump.
- Keep tests with the code they cover in the same commit.
- Conventional commit messages (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`,
  `chore:`, `ci:`). Never add AI attribution.
- Don't mix unrelated changes; if you notice an opportunistic cleanup, give it its
  own commit.
