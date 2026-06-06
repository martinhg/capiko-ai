---
name: issue-creation
description: "Create clear, actionable GitHub issues. Trigger: filing a bug report or feature request."
---

## Issue-first

Before opening an issue, check it does not already exist (`gh issue list --search`).
One issue = one problem or one request; don't bundle several.

## Bug report

- **Title**: the symptom in one line ("`capiko-ai version` prints a pseudo-version").
- **Steps to reproduce**: exact commands.
- **Expected vs actual**.
- **Environment**: OS, Go version, capiko version, how it was built/installed.
- Minimal repro beats a long description.

## Feature request

- **Problem** first (what's painful today), not the solution.
- **Proposed approach** + alternatives with trade-offs.
- **Scope**: what's in and explicitly out.

## Style

Short, concrete, skimmable. Use `gh issue create --title ... --body ...`.
Label appropriately. No AI attribution.
