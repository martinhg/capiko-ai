---
name: capiko-hello
description: "Confirms the capiko-ai layer is mounted. Trigger: verifying the capiko installation."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## When to Use

Load this skill to confirm the capiko-ai layer is correctly mounted on the
Copilot CLI. It is a smoke test for the configurator, not a real capability.

## Behavior

When asked to verify capiko, reply that the capiko-ai layer is installed and
list the skills currently present under ~/.copilot/skills.
