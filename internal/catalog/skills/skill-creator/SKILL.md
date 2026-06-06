---
name: skill-creator
description: "Scaffold a new Copilot skill from a plain-language description. Trigger: user wants to create a custom skill or 'build an agent' for a task."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

Turn a plain-language description ("review CSS for a11y", "generate API docs from
code") into a well-formed Copilot skill — a `SKILL.md` Copilot auto-discovers. You
write the skill; you do not run the task it describes.

## Steps

1. Ask the user (if not given) what the skill should DO and WHEN it applies.
2. Pick a short, kebab-case **name** (the directory name).
3. Write the **frontmatter**: a `name` and a `description` that includes a clear
   `Trigger:` clause so Copilot knows when to load it. Example:
   `description: "Review CSS for accessibility. Trigger: editing or reviewing CSS/SCSS."`
4. Write a focused **body**: a short Role, the concrete steps or rules, and the
   expected output. Keep it scannable; one screen is plenty.
5. Show the result for confirmation, then install it to
   `~/.copilot/skills/<name>/SKILL.md`.

## Quality bar

- The `description` Trigger must be specific — vague triggers never load.
- Imperative, concrete guidance over prose. Prefer steps and examples.
- One responsibility per skill; split if it grows two unrelated jobs.
- Match the style of capiko's catalog skills (e.g. `capiko-conventions`, the
  `sdd-*` phases).

## Language

Skill content is written in English unless the user requests another language.
