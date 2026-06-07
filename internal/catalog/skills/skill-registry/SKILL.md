---
name: skill-registry
description: "Resolve the installed-skill index so a delegator can inject the right SKILL.md paths into a sub-agent. Trigger: before delegating to a sub-agent, or when the user asks to list/index available skills."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

You are a delegator about to hand work to a sub-agent. Before you delegate, resolve
which skills that sub-agent needs and pass it the exact `SKILL.md` paths — so it loads
the full skill (the author's runtime contract) before doing any work. This registry is
an **index, not a summary**: never replace a `SKILL.md` with your own paraphrase.

This skill is for delegators only. If you are the sub-agent doing the work, you do not
run this — you read the `SKILL.md` paths your delegator handed you.

## Steps

1. Get the current index. Prefer the native engine — run:

   ```
   capiko-ai skill-registry
   ```

   It scans `~/.copilot/skills` (user scope) and `<cwd>/.copilot/skills` (project scope)
   and prints a markdown table of every installed skill by trigger/description, scope,
   and absolute `SKILL.md` path. Always fresh, never a stale cached file.

2. If the `capiko-ai` binary is unavailable, fall back to scanning `~/.copilot/skills`
   (and `<cwd>/.copilot/skills` if present) yourself: each `<name>/SKILL.md` is a skill;
   read its frontmatter `description` for the trigger.

3. Match the skills whose **trigger** fits the work you are about to delegate — by the
   files the sub-agent will touch (extensions, paths) and by the task (review, testing,
   PR creation, docs, etc.). Multiple skills can apply.

4. In the sub-agent handoff, add a `## Skills to load before work` section listing the
   exact `SKILL.md` paths from the index. Instruct the sub-agent to read those files
   before starting.

## Rules

- Pass **paths**, not generated summaries. The `SKILL.md` is the source of truth.
- Match by trigger, not by name guessing — the description carries the trigger.
- Re-run `capiko-ai skill-registry` rather than trusting a remembered list; skills can be
  installed or removed between delegations.
- If no skill matches, delegate without a skills section — do not force an irrelevant one.
