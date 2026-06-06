# capiko-ai — Agent Skills Index

When working on **this repository**, load the relevant skill(s) BEFORE writing
any code. Copilot reads this file as custom instructions, so these conventions
apply automatically when developing capiko in this repo.

Naming: `capiko-*` skills are repo-specific; unprefixed skills are portable.

| Trigger | Skill | Path |
| --- | --- | --- |
| Any change to this repo (workflow, conventions, architecture) | capiko-dev | `skills/capiko-dev/SKILL.md` |
| Writing or reviewing Go / Bubbletea tests, golden files | go-testing | `skills/go-testing/SKILL.md` |
| Splitting work into commits | work-unit-commits | `skills/work-unit-commits/SKILL.md` |
| Opening a PR | branch-pr | `skills/branch-pr/SKILL.md` |
| A change is growing too large for one PR | chained-pr | `skills/chained-pr/SKILL.md` |
| Filing a bug report or feature request | issue-creation | `skills/issue-creation/SKILL.md` |
| Writing PR reviews / replies / feedback | comment-writer | `skills/comment-writer/SKILL.md` |
| Writing a README, guide, RFC, or doc | cognitive-doc-design | `skills/cognitive-doc-design/SKILL.md` |

How to use: check the trigger column, then read the matching `SKILL.md` before
starting the work.

These skills are for **developing capiko**. The skills capiko ships to users live
in `internal/catalog/skills/` (embedded in the binary).
